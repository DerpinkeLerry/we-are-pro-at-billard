package match

import (
	"poolarena/game-server/internal/physics"
	"time"
)

type PublicState struct {
	ID                 string                 `json:"id"`
	State              string                 `json:"state"`
	Players            [2]Player              `json:"players"`
	Balls              []physics.BallSnapshot `json:"balls"`
	Turn               int                    `json:"turn"`
	TurnNonce          string                 `json:"turnNonce"`
	Groups             [2]string              `json:"groups"`
	BallInHand         bool                   `json:"ballInHand"`
	BallInHandHeadOnly bool                   `json:"ballInHandHeadOnly"`
	ShotNumber         int                    `json:"shotNumber"`
	Fouls              [2]int                 `json:"fouls"`
	PocketedCounts     [2]int                 `json:"pocketedCounts"`
	Winner             int                    `json:"winner"`
	EndReason          string                 `json:"endReason,omitempty"`
	DurationMS         int64                  `json:"durationMs"`
	Break              bool                   `json:"break"`
}

func (g *Game) Public() PublicState {
	duration := time.Since(g.StartedAt).Milliseconds()
	if !g.FinishedAt.IsZero() {
		duration = g.FinishedAt.Sub(g.StartedAt).Milliseconds()
	}
	return PublicState{ID: g.ID, State: g.State, Players: g.Players, Balls: physics.SnapshotBalls(g.Balls), Turn: g.Turn, TurnNonce: g.TurnNonce, Groups: [2]string{string(g.RuleState.Groups[0]), string(g.RuleState.Groups[1])}, BallInHand: g.BallInHand, BallInHandHeadOnly: g.BallInHandHeadOnly, ShotNumber: g.ShotNumber, Fouls: g.Fouls, PocketedCounts: g.Pocketed, Winner: g.Winner, EndReason: g.EndReason, DurationMS: duration, Break: g.RuleState.Break}
}
