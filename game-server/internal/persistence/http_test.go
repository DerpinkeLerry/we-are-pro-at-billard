package persistence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"poolarena/game-server/internal/match"
	"testing"
	"time"
)

func TestHTTPStorePingAndBeginMatch(t *testing.T) {
	secret := "01234567890123456789012345678901"
	seenPing := false
	seenBegin := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Secret") != secret {
			t.Fatalf("missing internal secret")
		}
		switch r.URL.Path {
		case "/internal/persistence/ping":
			seenPing = true
			w.WriteHeader(http.StatusOK)
		case "/internal/persistence/begin-match":
			seenBegin = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body["matchId"] != "match-1" || body["lobbyId"] != "lobby-1" {
				t.Fatalf("unexpected payload: %#v", body)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	store := OpenHTTP(ts.URL, secret)
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	g := &match.Game{
		ID:        "match-1",
		LobbyID:   "lobby-1",
		RackSeed:  42,
		StartedAt: time.Unix(100, 0).UTC(),
		Players: [2]match.Player{
			{Principal: "guest:a", Nickname: "A", CueSkin: "classic-maple"},
			{Principal: "guest:b", Nickname: "B", CueSkin: "dark-walnut"},
		},
	}
	if err := store.BeginMatch(ctx, g, "rules-v1", "physics-v1", "table-v1"); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if !seenPing || !seenBegin {
		t.Fatalf("expected both requests, ping=%v begin=%v", seenPing, seenBegin)
	}
}
