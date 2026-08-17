package physics

import (
	"math"
	"poolarena/game-server/internal/config"
	"testing"
)

func testEngine(t *testing.T) (config.All, *Engine) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg, NewEngine(cfg.Table, cfg.Physics)
}

func TestPocketCaptureRequiresThroatCrossing(t *testing.T) {
	cfg, e := testEngine(t)
	if len(e.Geometry.Pockets) != 6 {
		t.Fatalf("expected 6 pockets, got %d", len(e.Geometry.Pockets))
	}
	for _, pk := range e.Geometry.Pockets {
		// At the mouth the ball is still supported and must not be captured.
		if _, ok := e.Geometry.pocketForFalling(pk.MouthMid); ok {
			t.Fatalf("%s pocket captured ball at mouth", pk.Kind)
		}
		// Beyond the throat center, with the center line aligned, support is gone.
		p := pk.ThroatMid.Add(pk.Dir.Mul(cfg.Table.Ball.Radius * 0.2))
		got, ok := e.Geometry.pocketForFalling(p)
		if !ok || got.ID != pk.ID {
			t.Fatalf("%s pocket failed to capture beyond throat", pk.Kind)
		}
		// A ball grazing the mouth near a jaw must remain on the table side.
		grazing := pk.MouthMid.Add(pk.Tangent.Mul(pk.MouthWidth/2 - cfg.Table.Ball.Radius*0.15))
		if _, ok := e.Geometry.pocketForFalling(grazing); ok {
			t.Fatalf("%s pocket captured jaw-grazing path", pk.Kind)
		}
	}
}

func TestPocketGeometryHasDistinctCornerAndSideShapes(t *testing.T) {
	cfg, e := testEngine(t)
	corner, side := 0, 0
	for _, pk := range e.Geometry.Pockets {
		switch pk.Kind {
		case "corner":
			corner++
			if math.Abs(pk.MouthWidth-cfg.Table.Pockets.Corner.Mouth) > 1e-9 {
				t.Fatal("corner mouth does not use central config")
			}
		case "side":
			side++
			if math.Abs(pk.MouthWidth-cfg.Table.Pockets.Side.Mouth) > 1e-9 {
				t.Fatal("side mouth does not use central config")
			}
		}
	}
	if corner != 4 || side != 2 {
		t.Fatalf("unexpected pocket split corner=%d side=%d", corner, side)
	}
	if cfg.Table.Pockets.Corner.HorizontalCutDeg == cfg.Table.Pockets.Side.HorizontalCutDeg {
		t.Fatal("corner and side jaw cuts must differ")
	}
}

func TestJawCollisionReflectsBall(t *testing.T) {
	cfg, e := testEngine(t)
	var jaw Segment
	found := false
	for _, s := range e.Geometry.Segments {
		if s.Kind == "jaw" {
			jaw, found = s, true
			break
		}
	}
	if !found {
		t.Fatal("no jaw segment")
	}
	mid := jaw.A.Add(jaw.B).Mul(0.5)
	dir := jaw.B.Sub(jaw.A).Normalized()
	n := dir.Perp()
	b := Ball{ID: 0, Pos: mid.Add(n.Mul(cfg.Table.Ball.Radius * 0.9)), Vel: n.Mul(-1), Z: cfg.Table.Ball.Radius, State: BallOnTable}
	rep := ShotReport{}
	e.solveSegments([]Ball{b}, 0.1, &rep, map[int]bool{}, 0)
	if len(rep.Events) == 0 || rep.Events[0].Type != "jaw" {
		t.Fatalf("expected physical jaw collision, got %+v", rep.Events)
	}
}

func TestHighSpeedShotStillDetectsObjectBall(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	balls := []Ball{
		{ID: 0, Pos: Vec2{-0.75, 0.22}, Z: r, State: BallOnTable},
		{ID: 1, Pos: Vec2{0.15, 0.22}, Z: r, State: BallOnTable},
	}
	sim, err := e.SimulateShot(balls, ShotRequest{AimAngle: 0, Power: 1})
	if err != nil {
		t.Fatal(err)
	}
	if sim.Report.FirstObjectBall != 1 {
		t.Fatalf("high-speed collision tunneled; first=%d", sim.Report.FirstObjectBall)
	}
}

func TestSlidingTransitionsToRest(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	balls := []Ball{{ID: 0, Pos: Vec2{-0.5, 0.2}, Z: r, State: BallOnTable}}
	sim, err := e.SimulateShot(balls, ShotRequest{AimAngle: 0, Power: 0.25, CueOffsetY: -0.65})
	if err != nil {
		t.Fatal(err)
	}
	final := sim.Final[0]
	if final.Vel.Len() != 0 || final.Omega.Len() != 0 {
		t.Fatalf("ball did not enter stable resting state: v=%v omega=%v", final.Vel, final.Omega)
	}
	if sim.Report.SimulationDuration >= 25 {
		t.Fatal("rest detector failed before simulation cap")
	}
}

func TestJawCutsMatchConfiguredHorizontalAngles(t *testing.T) {
	cfg, e := testEngine(t)
	// Segments 0..5 are cushions. For the first (bottom-left) corner pocket,
	// segment 6 starts at the vertical rail mouth and points into the pocket.
	cornerJaw := e.Geometry.Segments[6]
	v := cornerJaw.B.Sub(cornerJaw.A)
	cornerAngle := math.Acos(v.Y/v.Len()) * 180 / math.Pi // angle from +Y rail direction
	if math.Abs(cornerAngle-cfg.Table.Pockets.Corner.HorizontalCutDeg) > 0.05 {
		t.Fatalf("corner jaw cut %.3f != configured %.3f", cornerAngle, cfg.Table.Pockets.Corner.HorizontalCutDeg)
	}
	// After eight corner jaw segments, the first side jaw is bottom-middle,
	// right-hand mouth endpoint; its adjacent rail runs in +X away from pocket.
	sideJaw := e.Geometry.Segments[14]
	v = sideJaw.B.Sub(sideJaw.A)
	sideAngle := math.Acos(v.X/v.Len()) * 180 / math.Pi
	if math.Abs(sideAngle-cfg.Table.Pockets.Side.HorizontalCutDeg) > 0.05 {
		t.Fatalf("side jaw cut %.3f != configured %.3f", sideAngle, cfg.Table.Pockets.Side.HorizontalCutDeg)
	}
}

func TestShallowPocketEntryCanRattleBackOut(t *testing.T) {
	cfg, e := testEngine(t)
	pk := e.Geometry.Pockets[0]
	b := Ball{ID: 1, Pos: pk.ThroatMid.Add(pk.Dir.Mul(0.001)), Vel: pk.Dir.Mul(-0.8), Z: cfg.Table.Ball.Radius * 0.9, VZ: -0.01, State: BallFalling, PocketID: pk.ID}
	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	balls := []Ball{b}
	e.integrate(balls, 0.02, 0.02, &rep)
	if balls[0].State != BallOnTable {
		t.Fatalf("expected shallow entry to escape, got state=%s z=%.4f", balls[0].State, balls[0].Z)
	}
	if len(rep.Pocketed) != 0 {
		t.Fatalf("rattle-back must not count as pocketed: %+v", rep.Pocketed)
	}
}
