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

func TestSevenFootTableUsesExactTwoToOnePlayingSurface(t *testing.T) {
	cfg, _ := testEngine(t)
	if cfg.Table.Version != "pool-7ft-v2" {
		t.Fatalf("unexpected table version %q", cfg.Table.Version)
	}
	if math.Abs(cfg.Table.PlayingSurface.Length-1.9812) > 1e-9 || math.Abs(cfg.Table.PlayingSurface.Width-0.9906) > 1e-9 {
		t.Fatalf("expected 78x39 inch playing surface, got %.4fx%.4f m", cfg.Table.PlayingSurface.Length, cfg.Table.PlayingSurface.Width)
	}
	if math.Abs(cfg.Table.Rack.FootSpotX-cfg.Table.PlayingSurface.Length/4) > 1e-9 || math.Abs(cfg.Table.Rack.HeadStringX+cfg.Table.PlayingSurface.Length/4) > 1e-9 {
		t.Fatalf("spots are not at the table quarters: %+v", cfg.Table.Rack)
	}
}

func TestCueTipVerticalOffsetHasCorrectSpinDirection(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	initial := []Ball{{ID: 0, Pos: Vec2{-0.3, 0}, Z: r, State: BallOnTable}}
	top, err := e.SimulateShot(initial, ShotRequest{AimAngle: 0, Power: .15, CueOffsetY: .7})
	if err != nil {
		t.Fatal(err)
	}
	draw, err := e.SimulateShot(initial, ShotRequest{AimAngle: 0, Power: .15, CueOffsetY: -.7})
	if err != nil {
		t.Fatal(err)
	}
	if len(top.Frames) < 2 || top.Frames[1].Balls[0].WY <= 0 {
		t.Fatalf("positive tip height must create topspin, got %+v", top.Frames)
	}
	if len(draw.Frames) < 2 || draw.Frames[1].Balls[0].WY >= 0 {
		t.Fatalf("negative tip height must create draw, got %+v", draw.Frames)
	}
}

func TestPureRollingStaysOnNoSlipConstraint(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	b := Ball{ID: 0, Vel: Vec2{1, 0}, Omega: Vec3{Y: 1 / r}, Z: r, State: BallOnTable}
	e.integrateOnTable(&b, .1)
	slip := Vec2{b.Vel.X - r*b.Omega.Y, b.Vel.Y + r*b.Omega.X}.Len()
	if slip > 1e-10 {
		t.Fatalf("rolling resistance broke no-slip constraint: %.12f", slip)
	}
	expectedSpeed := 1 - cfg.Physics.RollingResistance*cfg.Physics.Gravity*.1
	if math.Abs(b.Vel.Len()-expectedSpeed) > 1e-10 {
		t.Fatalf("unexpected rolling deceleration %.9f, want %.9f", b.Vel.Len(), expectedSpeed)
	}
}

func TestGrazingCushionDoesNotStealTangentialSpeed(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	s := firstSegment(e.Geometry, "cushion")
	mid := s.A.Add(s.B).Mul(.5)
	balls := []Ball{{ID: 0, Pos: Vec2{mid.X, mid.Y - r*.99}, Vel: Vec2{2, .05}, Z: r, State: BallOnTable}}
	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	e.solveSegments(balls, .1, &rep, map[int]bool{}, -1)
	if loss := 2 - balls[0].Vel.X; loss < 0 || loss > .012 {
		t.Fatalf("grazing cushion changed tangential speed by %.6f: %+v", loss, balls[0].Vel)
	}
}

func TestPocketCaptureRequiresFullBallClearance(t *testing.T) {
	cfg, e := testEngine(t)
	pk := e.Geometry.Pockets[0]
	clearHalfWidth := pk.ThroatWidth/2 - cfg.Table.Ball.Radius
	p := pk.ThroatMid.Add(pk.Dir.Mul(.001)).Add(pk.Tangent.Mul(clearHalfWidth + .002))
	if _, ok := e.Geometry.pocketForFalling(p); ok {
		t.Fatal("ball intersecting a throat endpoint was captured")
	}
}

func TestBallImpactConservesMomentumAndDoesNotCreateEnergy(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	m := cfg.Table.Ball.Mass
	a := Ball{ID: 0, Vel: Vec2{1.8, .4}, Omega: Vec3{Z: 12}, Z: r, State: BallOnTable}
	b := Ball{ID: 1, Vel: Vec2{0, -.1}, Omega: Vec3{Z: -4}, Z: r, State: BallOnTable}
	momentumBefore := a.Vel.Add(b.Vel).Mul(m)
	energy := func(x Ball) float64 {
		inertia := .4 * m * r * r
		return .5*m*x.Vel.Len2() + .5*inertia*(x.Omega.X*x.Omega.X+x.Omega.Y*x.Omega.Y+x.Omega.Z*x.Omega.Z)
	}
	energyBefore := energy(a) + energy(b)
	if _, ok := e.resolveBallImpact(&a, &b, Vec2{1, 0}); !ok {
		t.Fatal("expected approaching balls to collide")
	}
	momentumAfter := a.Vel.Add(b.Vel).Mul(m)
	if momentumAfter.Sub(momentumBefore).Len() > 1e-10 {
		t.Fatalf("ball impact changed linear momentum: before=%+v after=%+v", momentumBefore, momentumAfter)
	}
	if energyAfter := energy(a) + energy(b); energyAfter > energyBefore+1e-10 {
		t.Fatalf("ball impact created energy: before=%.9f after=%.9f", energyBefore, energyAfter)
	}
}

func TestSevenFootBreakProducesStableRackSeparation(t *testing.T) {
	cfg, e := testEngine(t)
	initial := NewRack(cfg.Table, 17)
	start := make(map[int]Vec2, len(initial))
	for _, b := range initial {
		start[b.ID] = b.Pos
	}
	sim, err := e.SimulateShot(initial, ShotRequest{AimAngle: 0, Power: .78})
	if err != nil {
		t.Fatal(err)
	}
	if sim.Report.FirstObjectBall <= 0 {
		t.Fatalf("break missed the rack: first=%d", sim.Report.FirstObjectBall)
	}
	if sim.Report.SimulationDuration >= 25 {
		t.Fatal("break failed to settle before the simulation limit")
	}
	moved := 0
	for i, a := range sim.Final {
		if a.State == BallOffTable {
			t.Fatalf("ordinary break launched ball %d off table", a.ID)
		}
		if a.State != BallOnTable || a.Pos.Sub(start[a.ID]).Len() > cfg.Table.Ball.Diameter {
			moved++
		}
		if a.State != BallOnTable {
			continue
		}
		for j := i + 1; j < len(sim.Final); j++ {
			b := sim.Final[j]
			if b.State == BallOnTable && b.Pos.Sub(a.Pos).Len() < cfg.Table.Ball.Diameter-cfg.Physics.PenetrationSlop*2 {
				t.Fatalf("final balls overlap: %d and %d", a.ID, b.ID)
			}
		}
	}
	if moved < 6 {
		t.Fatalf("break scattered only %d balls", moved)
	}
}

func TestStationarySideSpinCannotBlockNextTurn(t *testing.T) {
	cfg, e := testEngine(t)
	r := cfg.Table.Ball.Radius
	balls := []Ball{{ID: 0, Pos: Vec2{0, 0}, Omega: Vec3{Z: 350}, Z: r, State: BallOnTable}}
	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	dt := 1.0 / float64(cfg.Physics.Hz)
	elapsed := 0.0
	for elapsed < 3 && !e.allResting(balls) {
		e.integrate(balls, dt, elapsed+dt, &rep)
		elapsed += dt
	}
	if !e.allResting(balls) {
		t.Fatalf("stationary side spin still blocked turn after %.2fs: omega=%+v", elapsed, balls[0].Omega)
	}
	if elapsed > 2.2 {
		t.Fatalf("stationary side spin took too long to settle: %.3fs", elapsed)
	}
}
