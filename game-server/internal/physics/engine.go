package physics

import (
	"errors"
	"math"
	"poolarena/game-server/internal/config"
	"sort"
)

type Engine struct {
	Table    config.Table
	Cfg      config.Physics
	Geometry TableGeometry
}

func NewEngine(t config.Table, p config.Physics) *Engine {
	return &Engine{Table: t, Cfg: p, Geometry: BuildGeometry(t)}
}

func (e *Engine) SimulateShot(initial []Ball, shot ShotRequest) (Simulation, error) {
	if shot.Power < 0 || shot.Power > 1 || shot.CueOffsetX*shot.CueOffsetX+shot.CueOffsetY*shot.CueOffsetY > 1.000001 {
		return Simulation{}, errors.New("invalid shot")
	}
	balls := cloneBalls(initial)
	cue := findBall(balls, 0)
	if cue == nil || cue.State != BallOnTable {
		return Simulation{}, errors.New("cue ball unavailable")
	}
	// Pull-back power is deliberately progressive. A linear mapping made short
	// mouse movements produce disproportionately hard shots, while still
	// offering no extra control for touch shots. Full pull-back remains the
	// configured break speed.
	power := math.Pow(shot.Power, e.Cfg.CuePowerExponent)
	speed := e.Cfg.CueMinSpeed + (e.Cfg.CueMaxSpeed-e.Cfg.CueMinSpeed)*power
	dir := Vec2{math.Cos(shot.AimAngle), math.Sin(shot.AimAngle)}
	cue.Vel = dir.Mul(speed)
	// A positive vertical tip offset is topspin: k x shotDirection points
	// along the positive rolling axis. Horizontal tip offset creates side spin.
	tang := dir.Perp()
	spinScale := speed / e.Table.Ball.Radius * e.Cfg.CueSpinFactor
	cue.Omega.X = tang.X * shot.CueOffsetY * spinScale
	cue.Omega.Y = tang.Y * shot.CueOffsetY * spinScale
	cue.Omega.Z = -shot.CueOffsetX * spinScale

	rep := ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	frames := []Frame{{Time: 0, Balls: SnapshotBalls(balls)}}
	dt := 1 / float64(e.Cfg.Hz)
	simTime := 0.0
	nextFrame := 1.0 / 30.0
	maxTime := 25.0
	railSeen := map[int]bool{}
	firstContactTime := -1.0
	for simTime < maxTime {
		maxSpeed := 0.0
		for i := range balls {
			if balls[i].State == BallOnTable {
				if s := balls[i].Vel.Len(); s > maxSpeed {
					maxSpeed = s
				}
			}
		}
		sub := 1
		allowed := e.Table.Ball.Radius * e.Cfg.MaxDisplacementFractionOfRadius
		if allowed > 0 {
			sub = int(math.Ceil(maxSpeed * dt / allowed))
			if sub < 1 {
				sub = 1
			}
			if sub > e.Cfg.MaxSubsteps {
				sub = e.Cfg.MaxSubsteps
			}
		}
		sdt := dt / float64(sub)
		for k := 0; k < sub; k++ {
			stepTime := simTime + float64(k+1)*sdt
			start := make([]Vec2, len(balls))
			for i := range balls {
				start[i] = balls[i].Pos
			}
			e.integrate(balls, sdt, stepTime, &rep)
			e.solveSweptBallContacts(balls, start, sdt, stepTime, &rep, &firstContactTime)
			e.solveBallContacts(balls, stepTime, &rep, &firstContactTime)
			e.solveSegments(balls, stepTime, &rep, railSeen, firstContactTime)
			e.checkPocketsAndBounds(balls, stepTime, &rep)
		}
		simTime += dt
		if simTime+1e-9 >= nextFrame {
			frames = append(frames, Frame{Time: simTime, Balls: SnapshotBalls(balls)})
			nextFrame += 1.0 / 30.0
		}
		if e.allResting(balls) {
			break
		}
	}
	rep.SimulationDuration = simTime
	for id := range railSeen {
		rep.RailBallIDs = append(rep.RailBallIDs, id)
	}
	sort.Ints(rep.RailBallIDs)
	if len(frames) == 0 || math.Abs(frames[len(frames)-1].Time-simTime) > 1e-6 {
		frames = append(frames, Frame{Time: simTime, Balls: SnapshotBalls(balls)})
	}
	return Simulation{Frames: frames, Final: balls, Report: rep}, nil
}

func (e *Engine) integrate(balls []Ball, dt, eventTime float64, rep *ShotReport) {
	r := e.Table.Ball.Radius
	g := e.Cfg.Gravity
	for i := range balls {
		b := &balls[i]
		switch b.State {
		case BallOnTable:
			e.integrateOnTable(b, dt)
			spinDecayRate := e.Cfg.SpinDecay
			if b.Vel.Len() < e.Cfg.SleepLinearSpeed*4 {
				spinDecayRate = e.Cfg.StationarySpinDecay
			}
			spinDecay := math.Exp(-spinDecayRate * dt)
			b.Omega.Z *= spinDecay
			b.Z = r
			horizontalSpin := math.Hypot(b.Omega.X, b.Omega.Y)
			if b.Vel.Len() < e.Cfg.SleepLinearSpeed && horizontalSpin < e.Cfg.SleepAngularSpeed && math.Abs(b.Omega.Z) < e.Cfg.SleepSideSpinSpeed {
				b.SleepFor += dt
			} else {
				b.SleepFor = 0
			}
			if b.SleepFor >= e.Cfg.SleepDuration {
				b.Vel = Vec2{}
				b.Omega = Vec3{}
			}
		case BallFalling:
			pk, ok := e.Geometry.PocketByID(b.PocketID)
			if !ok {
				b.State = BallOffTable
				b.Vel = Vec2{}
				b.VZ = 0
				continue
			}
			b.VZ -= g * dt
			b.Z += b.VZ * dt
			b.Vel = b.Vel.Mul(math.Exp(-e.Cfg.PocketHorizontalDamping * dt))
			b.Pos = b.Pos.Add(b.Vel.Mul(dt))
			b.Omega.Z *= math.Exp(-2 * dt)

			// A WPA pocket has vertical back draft rather than a cylindrical trigger.
			// As the ball drops, the liner opening expands by tan(backDraft)*depth.
			rel := b.Pos.Sub(pk.ThroatMid)
			depth := rel.Dot(pk.Dir)
			lat := rel.Dot(pk.Tangent)
			verticalDrop := math.Max(0, r-b.Z)
			extra := verticalDrop * math.Tan(pk.BackDraftDeg*math.Pi/180)
			progress := math.Min(1, verticalDrop/math.Max(pk.DropDepth, 1e-6))
			halfWidth := pk.ThroatWidth/2 + (pk.DropRX-pk.ThroatWidth/2)*progress + extra
			if math.Abs(lat) > halfWidth {
				sign := 1.0
				if lat < 0 {
					sign = -1
				}
				lat = sign * halfWidth
				vlat := b.Vel.Dot(pk.Tangent)
				if vlat*sign > 0 {
					b.Vel = b.Vel.Sub(pk.Tangent.Mul(1.35 * vlat))
				}
			}
			maxDepth := pk.DropRY + extra
			if depth > maxDepth {
				depth = maxDepth
				vdepth := b.Vel.Dot(pk.Dir)
				if vdepth > 0 {
					b.Vel = b.Vel.Sub(pk.Dir.Mul(1.28 * vdepth))
				}
			}
			b.Pos = pk.ThroatMid.Add(pk.Dir.Mul(depth)).Add(pk.Tangent.Mul(lat))

			// A shallow lip/rattle can still escape back across the throat. Nothing
			// is counted as pocketed until the ball has dropped irreversibly.
			if depth < -r*0.15 && b.Z > r*0.25 {
				b.State = BallOnTable
				b.Z = r
				b.VZ = 0
				b.PocketID = -1
				b.SleepFor = 0
				continue
			}
			pocketedZ := -pk.DropDepth + r*0.40
			if b.Z < pocketedZ {
				b.State = BallPocketed
				b.Vel = Vec2{}
				b.VZ = 0
				b.Omega = Vec3{}
				rep.Pocketed = append(rep.Pocketed, b.ID)
				rep.PocketByBall[b.ID] = pk.ID
				if b.ID == 0 {
					rep.CueScratch = true
				}
				rep.Events = append(rep.Events, Event{Time: eventTime, Type: "pocket", A: b.ID, PocketID: pk.ID, Intensity: 0.45})
			}
		}
	}
}

// integrateOnTable advances cloth contact without numerically stepping across
// the sliding-to-rolling transition. For a solid sphere the contact slip loses
// speed at 7/2 times the translational friction acceleration. Once it reaches
// zero, rolling resistance scales linear and rolling angular velocity together,
// preserving the no-slip constraint instead of pulling omega toward it with an
// arbitrary blend factor.
func (e *Engine) integrateOnTable(b *Ball, dt float64) {
	r := e.Table.Ball.Radius
	g := e.Cfg.Gravity
	remaining := dt
	slip := Vec2{b.Vel.X - r*b.Omega.Y, b.Vel.Y + r*b.Omega.X}
	slipSpeed := slip.Len()
	if slipSpeed > e.Cfg.SlipSpeedEpsilon {
		slideTime := remaining
		timeToRoll := slipSpeed / (3.5 * e.Cfg.SlidingFriction * g)
		if timeToRoll < slideTime {
			slideTime = timeToRoll
		}
		a := slip.Mul(-e.Cfg.SlidingFriction * g / slipSpeed)
		oldVel := b.Vel
		b.Vel = b.Vel.Add(a.Mul(slideTime))
		b.Omega.X += (5.0 / (2 * r)) * a.Y * slideTime
		b.Omega.Y -= (5.0 / (2 * r)) * a.X * slideTime
		b.Pos = b.Pos.Add(oldVel.Add(b.Vel).Mul(0.5 * slideTime))
		remaining -= slideTime
		if remaining <= 1e-12 {
			return
		}
	}

	// Snap only inside the small no-slip tolerance. At this point these values
	// are the exact rolling constraint, not an artificial source of energy.
	b.Omega.X = -b.Vel.Y / r
	b.Omega.Y = b.Vel.X / r
	speed := b.Vel.Len()
	if speed <= 0 {
		return
	}
	newSpeed := math.Max(0, speed-e.Cfg.RollingResistance*g*remaining)
	oldVel := b.Vel
	b.Vel = b.Vel.Mul(newSpeed / speed)
	b.Pos = b.Pos.Add(oldVel.Add(b.Vel).Mul(0.5 * remaining))
	b.Omega.X = -b.Vel.Y / r
	b.Omega.Y = b.Vel.X / r
}

func (e *Engine) solveSweptBallContacts(balls []Ball, start []Vec2, dt, eventTime float64, rep *ShotReport, firstTime *float64) {
	if dt <= 0 {
		return
	}
	r := e.Table.Ball.Radius
	minDist := 2 * r
	minDist2 := minDist * minDist
	for i := 0; i < len(balls); i++ {
		if balls[i].State != BallOnTable {
			continue
		}
		for j := i + 1; j < len(balls); j++ {
			if balls[j].State != BallOnTable {
				continue
			}
			// Ordinary overlap solving handles contacts that still overlap at the
			// end of the substep. The sweep is specifically for a pair that would
			// otherwise cross completely between two samples.
			if start[j].Sub(start[i]).Len2() <= minDist2 || balls[j].Pos.Sub(balls[i].Pos).Len2() <= minDist2 {
				continue
			}
			moveA := balls[i].Pos.Sub(start[i])
			moveB := balls[j].Pos.Sub(start[j])
			toi, ok := SweptCircleTOI(start[i], moveA.Mul(1/dt), start[j], moveB.Mul(1/dt), r, dt)
			if !ok || toi <= 1e-9 || toi >= dt-1e-9 {
				continue
			}
			lambda := toi / dt
			pa := start[i].Add(moveA.Mul(lambda))
			pb := start[j].Add(moveB.Mul(lambda))
			n := pb.Sub(pa).Normalized()
			if n.Len2() < 0.5 {
				continue
			}
			impactSpeed, resolved := e.resolveBallImpact(&balls[i], &balls[j], n)
			if !resolved {
				continue
			}

			// Advance the remaining fraction using the post-impact velocities. A
			// subsequent overlap/segment pass resolves simultaneous contacts.
			remaining := dt - toi
			balls[i].Pos = pa.Add(balls[i].Vel.Mul(remaining))
			balls[j].Pos = pb.Add(balls[j].Vel.Mul(remaining))
			intensity := math.Min(1, impactSpeed/5)
			rep.Events = append(rep.Events, Event{Time: eventTime - remaining, Type: "ball", A: balls[i].ID, B: balls[j].ID, Intensity: intensity})
			if rep.FirstObjectBall < 0 {
				if balls[i].ID == 0 && balls[j].ID != 0 {
					rep.FirstObjectBall = balls[j].ID
					*firstTime = eventTime - remaining
				}
				if balls[j].ID == 0 && balls[i].ID != 0 {
					rep.FirstObjectBall = balls[i].ID
					*firstTime = eventTime - remaining
				}
			}
		}
	}
}

func (e *Engine) solveBallContacts(balls []Ball, t float64, rep *ShotReport, firstTime *float64) {
	r := e.Table.Ball.Radius
	minDist := 2 * r
	for iter := 0; iter < e.Cfg.SolverIterations; iter++ {
		changed := false
		for i := 0; i < len(balls); i++ {
			if balls[i].State != BallOnTable {
				continue
			}
			for j := i + 1; j < len(balls); j++ {
				if balls[j].State != BallOnTable {
					continue
				}
				d := balls[j].Pos.Sub(balls[i].Pos)
				dist := d.Len()
				if dist >= minDist || dist < 1e-10 {
					continue
				}
				n := d.Mul(1 / dist)
				impactSpeed, resolved := e.resolveBallImpact(&balls[i], &balls[j], n)
				if resolved {
					intensity := math.Min(1, impactSpeed/5)
					rep.Events = append(rep.Events, Event{Time: t, Type: "ball", A: balls[i].ID, B: balls[j].ID, Intensity: intensity})
					if rep.FirstObjectBall < 0 {
						if balls[i].ID == 0 && balls[j].ID != 0 {
							rep.FirstObjectBall = balls[j].ID
							*firstTime = t
						}
						if balls[j].ID == 0 && balls[i].ID != 0 {
							rep.FirstObjectBall = balls[i].ID
							*firstTime = t
						}
					}
				}
				pen := minDist - dist - e.Cfg.PenetrationSlop
				if pen > 0 {
					corr := n.Mul(pen * e.Cfg.PenetrationPercent / 2)
					balls[i].Pos = balls[i].Pos.Sub(corr)
					balls[j].Pos = balls[j].Pos.Add(corr)
				}
				changed = true
			}
		}
		if !changed {
			break
		}
	}
}

// resolveBallImpact applies a rigid-sphere impulse for equal-mass pool balls.
// The tangential effective mass is 7/m: 2/m translation plus 5/m rotation
// (r^2/I from each solid sphere). The previous 4.5/m denominator made spin
// transfer too strong and produced visibly bent collision paths.
func (e *Engine) resolveBallImpact(a, b *Ball, n Vec2) (float64, bool) {
	m := e.Table.Ball.Mass
	r := e.Table.Ball.Radius
	vn := b.Vel.Sub(a.Vel).Dot(n)
	if vn >= 0 {
		return 0, false
	}
	impactSpeed := -vn
	restitution := e.effectiveRestitution(e.Cfg.BallRestitution, impactSpeed)
	jn := (1 + restitution) * impactSpeed / (2 / m)
	normalImpulse := n.Mul(jn)
	a.Vel = a.Vel.Sub(normalImpulse.Mul(1 / m))
	b.Vel = b.Vel.Add(normalImpulse.Mul(1 / m))

	tangent := n.Perp()
	contactSlip := b.Vel.Sub(a.Vel).Dot(tangent) - r*(a.Omega.Z+b.Omega.Z)
	jt := -contactSlip / (7 / m)
	limit := e.Cfg.BallFriction * jn
	jt = math.Max(-limit, math.Min(limit, jt))
	tangentImpulse := tangent.Mul(jt)
	a.Vel = a.Vel.Sub(tangentImpulse.Mul(1 / m))
	b.Vel = b.Vel.Add(tangentImpulse.Mul(1 / m))
	a.Omega.Z -= 2.5 * jt / (m * r)
	b.Omega.Z -= 2.5 * jt / (m * r)
	return impactSpeed, true
}

func (e *Engine) effectiveRestitution(base, impactSpeed float64) float64 {
	threshold := e.Cfg.RestitutionVelocityThreshold
	if threshold > 0 && impactSpeed < threshold {
		return base * impactSpeed / threshold
	}
	return base
}

func (e *Engine) solveSegments(balls []Ball, t float64, rep *ShotReport, railSeen map[int]bool, firstTime float64) {
	r := e.Table.Ball.Radius
	for i := range balls {
		b := &balls[i]
		if b.State != BallOnTable {
			continue
		}
		for _, s := range e.Geometry.Segments {
			cp := closestPointSegment(b.Pos, s.A, s.B)
			delta := b.Pos.Sub(cp)
			dist := delta.Len()
			if dist >= r || dist < 1e-10 {
				continue
			}
			n := delta.Mul(1 / dist)
			vn := b.Vel.Dot(n)
			if vn >= 0 {
				b.Pos = b.Pos.Add(n.Mul(r - dist))
				continue
			}
			impactSpeed := -vn
			restitution := e.effectiveRestitution(e.Cfg.CushionRestitution, impactSpeed)
			normalDeltaV := (1 + restitution) * impactSpeed
			b.Vel = b.Vel.Add(n.Mul(normalDeltaV))
			tangent := n.Perp()
			vt := b.Vel.Dot(tangent) - b.Omega.Z*r
			// A stationary cushion has tangential effective mass 3.5m. Limit
			// the sticking impulse by Coulomb friction so a grazing rail contact
			// cannot remove a fixed percentage of tangential speed.
			deltaT := -vt / 3.5
			frictionLimit := e.Cfg.CushionFriction * normalDeltaV
			deltaT = math.Max(-frictionLimit, math.Min(frictionLimit, deltaT))
			b.Vel = b.Vel.Add(tangent.Mul(deltaT))
			b.Omega.Z -= 2.5 * deltaT / r
			b.Pos = b.Pos.Add(n.Mul(r - dist + 0.00001))
			intensity := math.Min(1, impactSpeed/5)
			rep.Events = append(rep.Events, Event{Time: t, Type: s.Kind, A: b.ID, Intensity: intensity})
			if s.Kind == "cushion" || s.Kind == "jaw" {
				railSeen[b.ID] = true
				if firstTime >= 0 && t+1e-7 >= firstTime {
					rep.AnyRailAfterFirst = true
				}
			}
		}
	}
}

func (e *Engine) checkPocketsAndBounds(balls []Ball, t float64, rep *ShotReport) {
	r := e.Table.Ball.Radius
	for i := range balls {
		b := &balls[i]
		if b.State != BallOnTable {
			continue
		}
		if pk, ok := e.Geometry.pocketForFalling(b.Pos); ok {
			b.State = BallFalling
			b.PocketID = pk.ID
			b.VZ = math.Min(-0.04, -0.02-b.Vel.Len()*0.03)
			b.SleepFor = 0
			continue
		}
		if math.Abs(b.Pos.X) > e.Geometry.HalfL+2*r || math.Abs(b.Pos.Y) > e.Geometry.HalfW+2*r {
			b.State = BallOffTable
			b.Vel = Vec2{}
			rep.BallsOffTable = append(rep.BallsOffTable, b.ID)
			if b.ID == 0 {
				rep.CueScratch = true
			}
		}
	}
}

func (e *Engine) allResting(balls []Ball) bool {
	for _, b := range balls {
		if b.State == BallFalling {
			return false
		}
		if b.State == BallOnTable && (b.Vel.Len() > 0 || b.Omega.Len() > 0) {
			return false
		}
	}
	return true
}
func cloneBalls(in []Ball) []Ball { out := make([]Ball, len(in)); copy(out, in); return out }
func findBall(b []Ball, id int) *Ball {
	for i := range b {
		if b[i].ID == id {
			return &b[i]
		}
	}
	return nil
}
