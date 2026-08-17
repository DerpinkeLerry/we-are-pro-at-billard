package match

import (
	"math"
	"poolarena/game-server/internal/config"
	"poolarena/game-server/internal/physics"
	"testing"
)

func newTestGame(t *testing.T) *Game {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	engine := physics.NewEngine(cfg.Table, cfg.Physics)
	return New("lobby", [2]Player{{Principal: "A", Nickname: "A"}, {Principal: "B", Nickname: "B"}}, 0, cfg, engine)
}

func TestShotRejectsWrongPlayerAndStaleNonce(t *testing.T) {
	g := newTestGame(t)
	if _, _, err := g.StartShot(1, ShotInput{TurnNonce: g.TurnNonce, Power: 0.3}); err == nil {
		t.Fatal("out-of-turn shot accepted")
	}
	if _, _, err := g.StartShot(0, ShotInput{TurnNonce: "old-turn", Power: 0.3}); err == nil {
		t.Fatal("stale turn nonce accepted")
	}
}

func TestShotRejectsManipulatedCueValues(t *testing.T) {
	g := newTestGame(t)
	if _, _, err := g.StartShot(0, ShotInput{TurnNonce: g.TurnNonce, Power: 1.2}); err == nil {
		t.Fatal("overpowered shot accepted")
	}
	if _, _, err := g.StartShot(0, ShotInput{TurnNonce: g.TurnNonce, Power: 0.3, CueOffsetX: 1, CueOffsetY: 1}); err == nil {
		t.Fatal("cue tip outside cue ball accepted")
	}
}

func TestBallInHandServerValidation(t *testing.T) {
	g := newTestGame(t)
	g.prepareBallInHand(false)
	object := ballByID(g.Balls, 1)
	if object == nil {
		t.Fatal("rack object ball missing")
	}
	if err := g.PlaceCueBall(g.Turn, object.Pos); err == nil {
		t.Fatal("overlapping ball-in-hand placement accepted")
	}
	pk := g.engine.Geometry.Pockets[0]
	if err := g.PlaceCueBall(g.Turn, pk.MouthMid); err == nil {
		t.Fatal("pocket-mouth ball-in-hand placement accepted")
	}
	if err := g.PlaceCueBall(g.Turn, physics.Vec2{X: -0.45, Y: 0.16}); err != nil {
		t.Fatalf("legal ball-in-hand placement rejected: %v", err)
	}
	if g.State != StateAwaitingShot || g.BallInHand {
		t.Fatalf("ball-in-hand did not transition to shot state: %s", g.State)
	}
}

func TestRejectsNonFiniteAndInvalidCall(t *testing.T) {
	g := newTestGame(t)
	_, _, err := g.StartShot(0, ShotInput{RequestID: "nan", TurnNonce: g.TurnNonce, AimAngle: 0, Power: math.NaN(), CalledPocket: -1})
	if err == nil {
		t.Fatal("expected non-finite power rejection")
	}
	_, _, err = g.StartShot(0, ShotInput{RequestID: "call", TurnNonce: g.TurnNonce, AimAngle: 0, Power: .2, CalledBall: 16, CalledPocket: 8})
	if err == nil {
		t.Fatal("expected invalid call rejection")
	}
}
