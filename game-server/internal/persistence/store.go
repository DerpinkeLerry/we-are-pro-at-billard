package persistence

import (
	"context"
	"poolarena/game-server/internal/match"
	"poolarena/game-server/internal/physics"
	"poolarena/game-server/internal/rules"
)

type Store interface {
	Ping(context.Context) error
	BeginMatch(context.Context, *match.Game, string, string, string) error
	RecordShot(context.Context, string, int, string, match.ShotInput, physics.Simulation, rules.Outcome) error
	FinishMatch(context.Context, *match.Game) error
	SaveCheckpoint(context.Context, string, any) error
	CloseLobby(context.Context, string) error
	Close() error
}
