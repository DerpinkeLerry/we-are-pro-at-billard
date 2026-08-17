package app

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"github.com/gorilla/websocket"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"poolarena/game-server/internal/auth"
	"poolarena/game-server/internal/lobby"
	"poolarena/game-server/internal/persistence"
	"poolarena/game-server/internal/protocol"
	"poolarena/game-server/internal/realtime"
	"strings"
	"sync"
	"time"
)

type Server struct {
	manager        *lobby.Manager
	validator      *auth.Validator
	store          persistence.Store
	internalSecret string
	origins        map[string]bool
	limiter        *ipLimiter
	upgrader       websocket.Upgrader
}

func NewServer(manager *lobby.Manager, validator *auth.Validator, store persistence.Store, internalSecret, allowedOrigins string) *Server {
	origins := map[string]bool{}
	for _, o := range strings.Split(allowedOrigins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins[o] = true
		}
	}
	s := &Server{manager: manager, validator: validator, store: store, internalSecret: internalSecret, origins: origins, limiter: newIPLimiter()}
	s.upgrader = websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, CheckOrigin: s.originAllowed}
	return s
}

func (s *Server) originAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	if s.origins[origin] {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.ws)
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/ping", s.ping)
	mux.HandleFunc("/internal/lobbies", s.internalLobbies)
	return mux
}

func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.limiter.Allow(ip) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(16 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var first protocol.ClientMessage
	if err = conn.ReadJSON(&first); err != nil || first.Type != "AUTH" {
		_ = conn.WriteJSON(map[string]any{"type": "AUTH_FAILED", "reason": "auth_required"})
		_ = conn.Close()
		return
	}
	claims, err := s.validator.Validate(first.Token)
	if err != nil {
		slog.Warn("websocket auth failed", "ip", ip, "error", err)
		_ = conn.WriteJSON(map[string]any{"type": "AUTH_FAILED", "reason": "invalid_token"})
		_ = conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(45 * time.Second)) })
	client := realtime.NewClient(claims.Sub, claims.LobbyID, conn)
	go client.WritePump()
	s.manager.Join(claims, client)
	slog.Info("websocket connected", "lobby", claims.LobbyCode)
	defer func() { s.manager.Disconnect(client); _ = conn.Close() }()
	for {
		var msg protocol.ClientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
		if msg.Type == "AUTH" || len(msg.Type) > 40 {
			continue
		}
		s.manager.Message(client, msg)
	}
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin != "" && s.originAllowed(r) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UnixMilli()})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	lobbies, connections := s.manager.Counts()
	db := "ok"
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		db = "degraded"
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "database": db, "activeLobbies": lobbies, "connections": connections})
}
func (s *Server) internalLobbies(w http.ResponseWriter, r *http.Request) {
	provided := r.Header.Get("X-Internal-Secret")
	if len(provided) != len(s.internalSecret) || subtle.ConstantTimeCompare([]byte(provided), []byte(s.internalSecret)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, 200, map[string]any{"lobbies": s.manager.Summaries()})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		return strings.TrimSpace(strings.Split(x, ",")[0])
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return h
	}
	return r.RemoteAddr
}

type ipLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newIPLimiter() *ipLimiter { return &ipLimiter{hits: map[string][]time.Time{}} }
func (l *ipLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	a := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if now.Sub(t) < time.Minute {
			a = append(a, t)
		}
	}
	if len(a) >= 30 {
		l.hits[ip] = a
		return false
	}
	a = append(a, now)
	l.hits[ip] = a
	return true
}
