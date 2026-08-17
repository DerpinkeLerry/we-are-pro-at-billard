package persistence

import (
	"context"
	"poolarena/game-server/internal/match"
	"poolarena/game-server/internal/physics"
	"poolarena/game-server/internal/rules"
)

type Memory struct{}

func (Memory) Ping(context.Context) error                                            { return nil }
func (Memory) BeginMatch(context.Context, *match.Game, string, string, string) error { return nil }
func (Memory) RecordShot(context.Context, string, int, string, match.ShotInput, physics.Simulation, rules.Outcome) error {
	return nil
}
func (Memory) FinishMatch(context.Context, *match.Game) error    { return nil }
func (Memory) SaveCheckpoint(context.Context, string, any) error { return nil }
func (Memory) CloseLobby(context.Context, string) error          { return nil }
func (Memory) Close() error                                      { return nil }
