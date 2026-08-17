package physics_test

import (
	"math"
	"poolarena/game-server/internal/config"
	"poolarena/game-server/internal/physics"
	"testing"
)

func load(t *testing.T) (config.All, *physics.Engine) {
	t.Helper()
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return c, physics.NewEngine(c.Table, c.Physics)
}

func TestSweptBallBall(t *testing.T) {
	toi, ok := physics.SweptCircleTOI(physics.Vec2{X: -1, Y: 0}, physics.Vec2{X: 20, Y: 0}, physics.Vec2{X: 0, Y: 0}, physics.Vec2{}, 0.028575, 0.1)
	if !ok || toi <= 0 || toi >= 0.1 {
		t.Fatalf("unexpected toi %v %v", toi, ok)
	}
}

func TestHeadOnBallCollisionTransfersEnergy(t *testing.T) {
	c, e := load(t)
	r := c.Table.Ball.Radius
	balls := []physics.Ball{{ID: 0, Pos: physics.Vec2{X: -0.3, Y: 0}, Z: r, State: physics.BallOnTable}, {ID: 1, Pos: physics.Vec2{X: 0, Y: 0}, Z: r, State: physics.BallOnTable}}
	sim, err := e.SimulateShot(balls, physics.ShotRequest{AimAngle: 0, Power: .45})
	if err != nil {
		t.Fatal(err)
	}
	if sim.Report.FirstObjectBall != 1 {
		t.Fatalf("first contact %d", sim.Report.FirstObjectBall)
	}
	if len(sim.Report.Events) == 0 {
		t.Fatal("no collisions recorded")
	}
}

func TestCushionRebound(t *testing.T) {
	c, e := load(t)
	r := c.Table.Ball.Radius
	balls := []physics.Ball{{ID: 0, Pos: physics.Vec2{X: 0.35, Y: 0}, Z: r, State: physics.BallOnTable}}
	sim, err := e.SimulateShot(balls, physics.ShotRequest{AimAngle: math.Pi / 2, Power: .35})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range sim.Report.Events {
		if ev.Type == "cushion" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected cushion event")
	}
}

func TestFrictionStopsBall(t *testing.T) {
	c, e := load(t)
	r := c.Table.Ball.Radius
	balls := []physics.Ball{{ID: 0, Pos: physics.Vec2{X: -0.5, Y: 0}, Z: r, State: physics.BallOnTable}}
	sim, err := e.SimulateShot(balls, physics.ShotRequest{AimAngle: 0, Power: .18, CueOffsetY: .5})
	if err != nil {
		t.Fatal(err)
	}
	b := sim.Final[0]
	if b.Vel.Len() != 0 {
		t.Fatalf("ball did not sleep: %.5f", b.Vel.Len())
	}
	if sim.Report.SimulationDuration <= 0 {
		t.Fatal("no simulation")
	}
}

func TestCornerPocketHasJawGeometry(t *testing.T) {
	c, _ := load(t)
	g := physics.BuildGeometry(c.Table)
	jaws := 0
	for _, s := range g.Segments {
		if s.Kind == "jaw" {
			jaws++
		}
	}
	if jaws != 12 {
		t.Fatalf("expected 12 jaw segments, got %d", jaws)
	}
	if len(g.Pockets) != 6 {
		t.Fatalf("expected six pockets")
	}
}

func TestSideAndCornerPocketDiffer(t *testing.T) {
	c, _ := load(t)
	g := physics.BuildGeometry(c.Table)
	var corner, side bool
	for _, p := range g.Pockets {
		if p.Kind == "corner" {
			corner = true
			if p.MouthWidth == c.Table.Pockets.Side.Mouth {
				t.Fatal("corner/side mouth unexpectedly equal")
			}
		}
		if p.Kind == "side" {
			side = true
		}
	}
	if !corner || !side {
		t.Fatal("missing pocket kinds")
	}
}

func TestSpinAffectsCushionPath(t *testing.T) {
	c, e := load(t)
	r := c.Table.Ball.Radius
	base := []physics.Ball{{ID: 0, Pos: physics.Vec2{X: 0, Y: 0}, Z: r, State: physics.BallOnTable}}
	a, _ := e.SimulateShot(base, physics.ShotRequest{AimAngle: math.Pi / 4, Power: .4, CueOffsetX: 0})
	b, _ := e.SimulateShot(base, physics.ShotRequest{AimAngle: math.Pi / 4, Power: .4, CueOffsetX: .7})
	if math.Abs(a.Final[0].Pos.X-b.Final[0].Pos.X) < 0.002 && math.Abs(a.Final[0].Pos.Y-b.Final[0].Pos.Y) < 0.002 {
		t.Fatal("side spin did not measurably change path")
	}
}
