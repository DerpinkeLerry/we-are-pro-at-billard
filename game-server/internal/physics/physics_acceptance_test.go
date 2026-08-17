package physics

import (
	"math"
	"testing"
)

func firstSegment(g TableGeometry, kind string) Segment {
	for _, s := range g.Segments {
		if s.Kind == kind {
			return s
		}
	}
	return Segment{}
}

func TestAngledBallCollisionDeflectsBothBalls(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	a := Ball{ID: 0, Pos: Vec2{-r * .90, -r * .20}, Vel: Vec2{1.8, .55}, Z: r, State: BallOnTable}
	b := Ball{ID: 1, Pos: Vec2{r * .90, r * .20}, Vel: Vec2{}, Z: r, State: BallOnTable}
	balls := []Ball{a, b}
	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	ft := -1.0
	e.solveBallContacts(balls, .1, &rep, &ft)
	if balls[1].Vel.Len() < .05 {
		t.Fatalf("angled collision failed to move object ball: %+v", balls[1].Vel)
	}
	if math.Abs(balls[1].Vel.Y) < .02 {
		t.Fatalf("angled collision lacked lateral component: %+v", balls[1].Vel)
	}
}

func TestTwoMovingBallsExchangeMomentum(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	balls := []Ball{
		{ID: 0, Pos: Vec2{-r * .96, 0}, Vel: Vec2{1.5, 0}, Z: r, State: BallOnTable},
		{ID: 1, Pos: Vec2{r * .96, 0}, Vel: Vec2{-.65, 0}, Z: r, State: BallOnTable},
	}
	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	ft := -1.0
	e.solveBallContacts(balls, .1, &rep, &ft)
	if balls[0].Vel.X >= 0 || balls[1].Vel.X <= 0 {
		t.Fatalf("expected opposing moving balls to reverse/exchange: a=%+v b=%+v", balls[0].Vel, balls[1].Vel)
	}
}

func TestPerpendicularCushionImpactLosesEnergy(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	s := firstSegment(e.Geometry, "cushion")
	mid := s.A.Add(s.B).Mul(.5)
	// First long-rail segment is at +Y. Put the ball just inside the rail and drive outward.
	balls := []Ball{{ID: 0, Pos: Vec2{mid.X, mid.Y - r*.92}, Vel: Vec2{0, 2.0}, Z: r, State: BallOnTable}}
	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	e.solveSegments(balls, .1, &rep, map[int]bool{}, -1)
	if balls[0].Vel.Y >= 0 {
		t.Fatalf("cushion did not rebound: %+v", balls[0].Vel)
	}
	if math.Abs(balls[0].Vel.Y) >= 2.0 {
		t.Fatalf("cushion failed to lose energy: %+v", balls[0].Vel)
	}
}

func TestAngledCushionPreservesTangentialMotion(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	s := firstSegment(e.Geometry, "cushion")
	mid := s.A.Add(s.B).Mul(.5)
	balls := []Ball{{ID: 0, Pos: Vec2{mid.X, mid.Y - r*.92}, Vel: Vec2{.9, 1.7}, Z: r, State: BallOnTable}}
	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	e.solveSegments(balls, .1, &rep, map[int]bool{}, -1)
	if balls[0].Vel.Y >= 0 || math.Abs(balls[0].Vel.X) < .2 {
		t.Fatalf("angled cushion response invalid: %+v", balls[0].Vel)
	}
}

func TestCenteredPocketEntryFallsWithoutAttraction(t *testing.T) {
	cfg, e := testEngine(t)
	pk := e.Geometry.Pockets[0]
	b := Ball{ID: 1, Pos: pk.ThroatMid.Add(pk.Dir.Mul(.001)), Vel: pk.Dir.Mul(.35), Z: cfg.Table.Ball.Radius, State: BallOnTable, PocketID: -1}
	balls := []Ball{b}
	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	e.checkPocketsAndBounds(balls, 0, &rep)
	if balls[0].State != BallFalling || balls[0].PocketID != pk.ID {
		t.Fatalf("centered throat crossing not captured: %+v", balls[0])
	}
	// Capture changes support state only. It does not teleport or pull the center sideways.
	if math.Abs(balls[0].Pos.Sub(b.Pos).Len()) > 1e-12 {
		t.Fatal("pocket entry attracted/teleported the ball")
	}
}

func TestHighSpeedJawGrazingStaysGeometric(t *testing.T) {
	cfg, e := testEngine(t)
	pk := e.Geometry.Pockets[0]
	r := cfg.Table.Ball.Radius
	// Center is still on the table side and near one jaw: capture must not happen merely due to proximity to pocket center.
	p := pk.MouthMid.Add(pk.Tangent.Mul(pk.MouthWidth/2 - r*.12)).Sub(pk.Dir.Mul(r * .18))
	if _, ok := e.Geometry.pocketForFalling(p); ok {
		t.Fatal("high-speed jaw-grazing position was treated as a pocket circle")
	}
}

func TestSlidingFrictionReducesSlipBeforeSleep(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	balls := []Ball{{ID: 0, Pos: Vec2{-0.45, .2}, Vel: Vec2{2.0, 0}, Omega: Vec3{}, Z: r, State: BallOnTable}}
	beforeSlip := math.Abs(balls[0].Vel.X - r*balls[0].Omega.Y)
	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	for i := 0; i < 20; i++ {
		e.integrate(balls, 1.0/float64(cfg.Physics.Hz), float64(i)/float64(cfg.Physics.Hz), &rep)
	}
	afterSlip := math.Abs(balls[0].Vel.X - r*balls[0].Omega.Y)
	if afterSlip >= beforeSlip {
		t.Fatalf("sliding friction did not reduce contact slip: before=%f after=%f", beforeSlip, afterSlip)
	}
}

func TestTopspinAndBackspinProduceDifferentTravel(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	initial := []Ball{{ID: 0, Pos: Vec2{-.45, .18}, Z: r, State: BallOnTable}}
	top, err := e.SimulateShot(initial, ShotRequest{AimAngle: 0, Power: .22, CueOffsetY: .75})
	if err != nil {
		t.Fatal(err)
	}
	draw, err := e.SimulateShot(initial, ShotRequest{AimAngle: 0, Power: .22, CueOffsetY: -.75})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(top.Final[0].Pos.X-draw.Final[0].Pos.X) < .008 {
		t.Fatalf("topspin/backspin paths too similar: top=%.4f draw=%.4f", top.Final[0].Pos.X, draw.Final[0].Pos.X)
	}
}

func TestSideSpinIsStoredAsVerticalAngularVelocity(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	balls := []Ball{{ID: 0, Pos: Vec2{-.4, .2}, Z: r, State: BallOnTable}}
	sim, err := e.SimulateShot(balls, ShotRequest{AimAngle: 0, Power: .18, CueOffsetX: .8})
	if err != nil {
		t.Fatal(err)
	}
	// The early network frame must expose real angular velocity rather than a cosmetic scalar.
	if len(sim.Frames) < 2 || math.Abs(sim.Frames[1].Balls[0].WZ) < .1 {
		t.Fatalf("side spin angular velocity missing from simulation: %+v", sim.Frames)
	}
}
