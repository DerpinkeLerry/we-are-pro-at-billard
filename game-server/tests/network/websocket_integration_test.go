//go:build integration

package network_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"poolarena/game-server/internal/app"
	"poolarena/game-server/internal/auth"
	"poolarena/game-server/internal/config"
	"poolarena/game-server/internal/lobby"
	"poolarena/game-server/internal/persistence"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const joinSecret = "integration-join-secret-32-bytes-minimum-value"
const internalSecret = "integration-internal-secret-32-bytes-minimum"

type testPeer struct {
	sub, nick string
	conn      *websocket.Conn
	publicID  string
}

func signToken(t *testing.T, sub, nick, jti string) string {
	t.Helper()
	now := time.Now().Unix()
	claims := map[string]any{
		"iss": "pool-web", "aud": "pool-game", "sub": sub, "iat": now, "nbf": now - 1, "exp": now + 60, "jti": jti,
		"principalType": "guest", "principalId": strings.TrimPrefix(sub, "guest:"), "nickname": nick,
		"lobbyId": "00000000-0000-4000-8000-000000000001", "lobbyCode": "NETTEST", "lobbyName": "Network Test",
		"cueSkin": "classic-maple", "shotTimerSeconds": 45, "rulesetVersion": "wpa-8ball-v1", "tableConfigVersion": "wpa-9ft-v1",
	}
	header, _ := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT"})
	body, _ := json.Marshal(claims)
	enc := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	unsigned := enc(header) + "." + enc(body)
	mac := hmac.New(sha256.New, []byte(joinSecret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + enc(mac.Sum(nil))
}

func startServer(t *testing.T, tune func(*config.All)) (string, *lobby.Manager, func()) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if tune != nil {
		tune(&cfg)
	}
	store := persistence.Memory{}
	manager := lobby.NewManager(cfg, store)
	srv := app.NewServer(manager, auth.NewValidator(joinSecret), store, internalSecret, "http://example.test")
	httpSrv := httptest.NewServer(srv.Routes())
	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"
	cleanup := func() {
		httpSrv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		manager.Shutdown(ctx)
	}
	return wsURL, manager, cleanup
}

func dial(t *testing.T, wsURL, token string) *websocket.Conn {
	t.Helper()
	h := http.Header{"Origin": []string{"http://example.test"}}
	c, _, err := websocket.DefaultDialer.Dial(wsURL, h)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.WriteJSON(map[string]any{"type": "AUTH", "token": token}); err != nil {
		t.Fatal(err)
	}
	return c
}

func join(t *testing.T, wsURL, sub, nick string, serial int) *testPeer {
	t.Helper()
	c := dial(t, wsURL, signToken(t, sub, nick, fmt.Sprintf("%s-%d-%d", nick, serial, time.Now().UnixNano())))
	m := waitFor(t, c, "AUTH_OK", 2*time.Second)
	data := mustData(t, m)
	id, _ := data["participantId"].(string)
	return &testPeer{sub: sub, nick: nick, conn: c, publicID: id}
}

func readMessage(t *testing.T, c *websocket.Conn, timeout time.Duration) map[string]any {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(timeout))
	for {
		mt, b, err := c.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if mt != websocket.TextMessage {
			continue
		}
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			return m
		}
	}
}
func waitFor(t *testing.T, c *websocket.Conn, kind string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m := readMessage(t, c, time.Until(deadline))
		if m["type"] == kind {
			return m
		}
	}
	t.Fatalf("did not receive %s", kind)
	return nil
}
func mustData(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	d, ok := m["data"].(map[string]any)
	if !ok {
		t.Fatalf("bad data %#v", m)
	}
	return d
}
func write(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	if err := c.WriteJSON(v); err != nil {
		t.Fatal(err)
	}
}

func readyAndStart(t *testing.T, a, b *testPeer) map[string]any {
	t.Helper()
	write(t, a.conn, map[string]any{"type": "READY_SET", "ready": true})
	write(t, b.conn, map[string]any{"type": "READY_SET", "ready": true})
	started := waitFor(t, a.conn, "MATCH_STARTED", 3*time.Second)
	_ = waitFor(t, b.conn, "MATCH_STARTED", 3*time.Second)
	return mustData(t, started)
}

func TestWebSocketRejectsBadToken(t *testing.T) {
	wsURL, _, cleanup := startServer(t, nil)
	defer cleanup()
	c := dial(t, wsURL, "not-a-jwt")
	defer c.Close()
	if m := waitFor(t, c, "AUTH_FAILED", 2*time.Second); m["type"] != "AUTH_FAILED" {
		t.Fatalf("unexpected %#v", m)
	}
}

func TestThreeConnectionsProduceTwoPlayersAndQueuedSpectator(t *testing.T) {
	wsURL, _, cleanup := startServer(t, nil)
	defer cleanup()
	a := join(t, wsURL, "guest:00000000-0000-4000-8000-000000000001", "Alpha", 1)
	defer a.conn.Close()
	b := join(t, wsURL, "guest:00000000-0000-4000-8000-000000000002", "Bravo", 1)
	defer b.conn.Close()
	c := join(t, wsURL, "guest:00000000-0000-4000-8000-000000000003", "Charlie", 1)
	defer c.conn.Close()
	state := mustData(t, waitFor(t, c.conn, "LOBBY_STATE", 2*time.Second))
	parts := state["participants"].([]any)
	players, spectators := 0, 0
	for _, raw := range parts {
		p := raw.(map[string]any)
		if p["role"] == "player" {
			players++
		}
		if p["role"] == "spectator" {
			spectators++
		}
	}
	if players != 2 || spectators != 1 {
		t.Fatalf("players=%d spectators=%d state=%#v", players, spectators, state)
	}
	if q := state["queue"].([]any); len(q) != 1 {
		t.Fatalf("expected queued spectator %#v", q)
	}
}

func TestSpectatorCannotShoot(t *testing.T) {
	wsURL, _, cleanup := startServer(t, nil)
	defer cleanup()
	a := join(t, wsURL, "guest:a", "Alpha", 1)
	defer a.conn.Close()
	b := join(t, wsURL, "guest:b", "Bravo", 1)
	defer b.conn.Close()
	c := join(t, wsURL, "guest:c", "Charlie", 1)
	defer c.conn.Close()
	write(t, c.conn, map[string]any{"type": "SHOT_REQUEST", "requestId": "spectator-shot", "matchId": "x", "turnNonce": "x", "power": .5, "calledPocket": -1})
	m := mustData(t, waitFor(t, c.conn, "SHOT_REJECTED", 2*time.Second))
	if m["reason"] != "spectator_cannot_shoot" {
		t.Fatalf("unexpected rejection %#v", m)
	}
}

func TestActivePlayerReconnectDoesNotDuplicateParticipant(t *testing.T) {
	wsURL, _, cleanup := startServer(t, func(c *config.All) { c.Rules.ReconnectGraceSeconds = 3 })
	defer cleanup()
	a := join(t, wsURL, "guest:reconnect-a", "Alpha", 1)
	b := join(t, wsURL, "guest:reconnect-b", "Bravo", 1)
	defer b.conn.Close()
	oldID := a.publicID
	_ = a.conn.Close()
	_ = waitFor(t, b.conn, "PLAYER_RECONNECTING", 2*time.Second)
	a2 := join(t, wsURL, a.sub, a.nick, 2)
	defer a2.conn.Close()
	if a2.publicID != oldID {
		t.Fatalf("reconnect created new participant: old=%s new=%s", oldID, a2.publicID)
	}
	state := mustData(t, waitFor(t, a2.conn, "LOBBY_STATE", 2*time.Second))
	if len(state["participants"].([]any)) != 2 {
		t.Fatalf("duplicate participant after reconnect %#v", state)
	}
}

func TestOutOfTurnShotRejectedAndShotSpamLimited(t *testing.T) {
	wsURL, _, cleanup := startServer(t, func(c *config.All) { c.Rules.CountdownSeconds = 0 })
	defer cleanup()
	a := join(t, wsURL, "guest:turn-a", "Alpha", 1)
	defer a.conn.Close()
	b := join(t, wsURL, "guest:turn-b", "Bravo", 1)
	defer b.conn.Close()
	m := readyAndStart(t, a, b)
	matchID := m["id"].(string)
	turnNonce := m["turnNonce"].(string)
	write(t, b.conn, map[string]any{"type": "SHOT_REQUEST", "requestId": "wrong-turn", "matchId": matchID, "turnNonce": turnNonce, "aimAngle": 0, "power": .2, "calledPocket": -1})
	rej := mustData(t, waitFor(t, b.conn, "SHOT_REJECTED", 2*time.Second))
	if rej["reason"] != "not_your_turn" {
		t.Fatalf("out-of-turn reason %#v", rej)
	}
	for i := 0; i < 9; i++ {
		write(t, a.conn, map[string]any{"type": "SHOT_REQUEST", "requestId": fmt.Sprintf("spam-%d", i), "matchId": "wrong", "turnNonce": turnNonce, "aimAngle": 0, "power": .2, "calledPocket": -1})
	}
	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		msg := readMessage(t, a.conn, time.Until(deadline))
		if msg["type"] == "SHOT_REJECTED" {
			d := mustData(t, msg)
			if d["reason"] == "shot_rate_limited" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("shot spam was not rate limited")
	}
}

func TestDisconnectForfeitEndsMatchAndRotatesQueue(t *testing.T) {
	wsURL, _, cleanup := startServer(t, func(c *config.All) {
		c.Rules.CountdownSeconds = 0
		c.Rules.ReconnectGraceSeconds = 1
		c.Rules.PostGameSeconds = 1
	})
	defer cleanup()
	a := join(t, wsURL, "guest:forfeit-a", "Alpha", 1)
	b := join(t, wsURL, "guest:forfeit-b", "Bravo", 1)
	defer b.conn.Close()
	c := join(t, wsURL, "guest:forfeit-c", "Charlie", 1)
	defer c.conn.Close()
	_ = readyAndStart(t, a, b)
	_ = a.conn.Close()
	_ = waitFor(t, b.conn, "PLAYER_RECONNECTING", 2*time.Second)
	finished := mustData(t, waitFor(t, b.conn, "MATCH_FINISHED", 3*time.Second))
	matchData := finished["match"].(map[string]any)
	if matchData["endReason"] != "disconnect_forfeit" {
		t.Fatalf("wrong match end %#v", matchData)
	}
	next := mustData(t, waitFor(t, c.conn, "NEXT_MATCH", 3*time.Second))
	parts := next["participants"].([]any)
	active := map[string]bool{}
	for _, raw := range parts {
		p := raw.(map[string]any)
		if p["role"] == "player" {
			active[p["nickname"].(string)] = true
		}
	}
	if !active["Bravo"] || !active["Charlie"] {
		t.Fatalf("expected Charlie vs Bravo after rotation: %#v", next)
	}
}

func TestEmptyLobbyCloses(t *testing.T) {
	wsURL, manager, cleanup := startServer(t, func(c *config.All) { c.Rules.ReconnectGraceSeconds = 0; c.Rules.EmptyLobbySeconds = 0 })
	defer cleanup()
	a := join(t, wsURL, "guest:empty-a", "Alpha", 1)
	_ = a.conn.Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n, _ := manager.Counts()
		if n == 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	n, _ := manager.Counts()
	t.Fatalf("empty lobby remained alive: %d", n)
}
