package persistence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"poolarena/game-server/internal/match"
	"poolarena/game-server/internal/physics"
	"poolarena/game-server/internal/rules"
	"strings"
	"time"
)

type HTTPStore struct {
	baseURL string
	secret  string
	client  *http.Client
}

func OpenHTTP(baseURL, secret string) *HTTPStore {
	return &HTTPStore{
		baseURL: strings.TrimRight(baseURL, "/"),
		secret:  secret,
		client:  &http.Client{Timeout: 3 * time.Second},
	}
}

func (h *HTTPStore) Ping(ctx context.Context) error {
	return h.do(ctx, http.MethodGet, "/internal/persistence/ping", nil)
}

func (h *HTTPStore) BeginMatch(ctx context.Context, g *match.Game, ruleset, physicsVersion, tableVersion string) error {
	players := make([]map[string]any, 0, 2)
	for _, pl := range g.Players {
		players = append(players, map[string]any{
			"principal": pl.Principal,
			"nickname":  pl.Nickname,
			"cueSkin":   pl.CueSkin,
		})
	}
	return h.do(ctx, http.MethodPost, "/internal/persistence/begin-match", map[string]any{
		"matchId":            g.ID,
		"lobbyId":            g.LobbyID,
		"rulesetVersion":     ruleset,
		"physicsVersion":     physicsVersion,
		"tableConfigVersion": tableVersion,
		"engineVersion":      "pool-arena-go-v1",
		"rackSeed":           g.RackSeed,
		"startedAt":          g.StartedAt,
		"players":            players,
	})
}

func (h *HTTPStore) RecordShot(ctx context.Context, matchID string, shotNumber int, principal string, in match.ShotInput, sim physics.Simulation, out rules.Outcome) error {
	raw, _ := json.Marshal(physics.SnapshotBalls(sim.Final))
	sum := sha256.Sum256(raw)
	return h.do(ctx, http.MethodPost, "/internal/persistence/record-shot", map[string]any{
		"matchId":              matchID,
		"shotNumber":           shotNumber,
		"principal":            principal,
		"aimAngle":             in.AimAngle,
		"power":                in.Power,
		"cueOffsetX":           in.CueOffsetX,
		"cueOffsetY":           in.CueOffsetY,
		"calledBall":           in.CalledBall,
		"calledPocket":         in.CalledPocket,
		"safety":               in.Safety,
		"startedAt":            time.Now().UTC(),
		"simulationDurationMs": int(sim.Report.SimulationDuration * 1000),
		"foulCode":             out.FoulCode,
		"finalStateHash":       hex.EncodeToString(sum[:]),
	})
}

func (h *HTTPStore) FinishMatch(ctx context.Context, g *match.Game) error {
	winner := ""
	loser := ""
	if g.Winner >= 0 {
		winner = g.Players[g.Winner].Principal
	}
	if g.Loser >= 0 {
		loser = g.Players[g.Loser].Principal
	}
	players := make([]map[string]any, 0, 2)
	for i, pl := range g.Players {
		result := "loss"
		if i == g.Winner {
			result = "win"
		}
		players = append(players, map[string]any{
			"seat":          i,
			"principal":     pl.Principal,
			"group":         string(g.RuleState.Groups[i]),
			"fouls":         g.Fouls[i],
			"ballsPocketed": g.Pocketed[i],
			"result":        result,
		})
	}
	return h.do(ctx, http.MethodPost, "/internal/persistence/finish-match", map[string]any{
		"matchId":         g.ID,
		"finishedAt":      g.FinishedAt,
		"winnerPrincipal": winner,
		"loserPrincipal":  loser,
		"endReason":       g.EndReason,
		"durationMs":      g.FinishedAt.Sub(g.StartedAt).Milliseconds(),
		"players":         players,
	})
}

func (h *HTTPStore) SaveCheckpoint(ctx context.Context, matchID string, state any) error {
	return h.do(ctx, http.MethodPost, "/internal/persistence/checkpoint", map[string]any{"matchId": matchID, "state": state})
}

func (h *HTTPStore) CloseLobby(ctx context.Context, lobbyID string) error {
	return h.do(ctx, http.MethodPost, "/internal/persistence/close-lobby", map[string]any{"lobbyId": lobbyID})
}

func (h *HTTPStore) Close() error { return nil }

func (h *HTTPStore) do(ctx context.Context, method, path string, payload any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Internal-Secret", h.secret)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("persistence HTTP %s: %s", path, resp.Status)
	}
	return nil
}
