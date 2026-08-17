package lobby

import (
	"context"
	"poolarena/game-server/internal/auth"
	"poolarena/game-server/internal/config"
	"poolarena/game-server/internal/persistence"
	"poolarena/game-server/internal/physics"
	"poolarena/game-server/internal/protocol"
	"poolarena/game-server/internal/realtime"
	"sync"
)

type Manager struct {
	mu      sync.RWMutex
	lobbies map[string]*Lobby
	cfg     config.All
	engine  *physics.Engine
	store   persistence.Store
}

func NewManager(cfg config.All, store persistence.Store) *Manager {
	return &Manager{lobbies: map[string]*Lobby{}, cfg: cfg, engine: physics.NewEngine(cfg.Table, cfg.Physics), store: store}
}

func (m *Manager) Join(c auth.Claims, client *realtime.Client) {
	m.mu.Lock()
	l := m.lobbies[c.LobbyID]
	if l == nil {
		l = New(c, m.cfg, m.engine, m.store)
		m.lobbies[c.LobbyID] = l
		go func() {
			l.Run()
			m.mu.Lock()
			if m.lobbies[c.LobbyID] == l {
				delete(m.lobbies, c.LobbyID)
			}
			m.mu.Unlock()
		}()
	}
	m.mu.Unlock()
	l.join <- joinCmd{claims: c, client: client}
}
func (m *Manager) Message(client *realtime.Client, msg protocol.ClientMessage) {
	m.mu.RLock()
	l := m.lobbies[client.LobbyID]
	m.mu.RUnlock()
	if l != nil {
		l.messages <- msgCmd{client: client, msg: msg}
	}
}
func (m *Manager) Disconnect(client *realtime.Client) {
	m.mu.RLock()
	l := m.lobbies[client.LobbyID]
	m.mu.RUnlock()
	if l != nil {
		l.leave <- leaveCmd{client: client}
	}
}
func (m *Manager) Summaries() []RuntimeSummary {
	m.mu.RLock()
	ls := make([]*Lobby, 0, len(m.lobbies))
	for _, l := range m.lobbies {
		ls = append(ls, l)
	}
	m.mu.RUnlock()
	out := make([]RuntimeSummary, 0, len(ls))
	for _, l := range ls {
		ch := make(chan RuntimeSummary, 1)
		l.summary <- summaryCmd{reply: ch}
		out = append(out, <-ch)
	}
	return out
}
func (m *Manager) Counts() (int, int) {
	s := m.Summaries()
	conns := 0
	for _, x := range s {
		conns += x.Players + x.Spectators
	}
	return len(s), conns
}
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.RLock()
	ls := make([]*Lobby, 0, len(m.lobbies))
	for _, l := range m.lobbies {
		ls = append(ls, l)
	}
	m.mu.RUnlock()
	for _, l := range ls {
		done := make(chan struct{})
		select {
		case l.shutdown <- shutdownCmd{done: done}:
		case <-ctx.Done():
			return
		}
		select {
		case <-done:
		case <-ctx.Done():
			return
		}
	}
}
