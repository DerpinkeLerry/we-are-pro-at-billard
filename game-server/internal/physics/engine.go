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
	speed := e.Cfg.CueMinSpeed + (e.Cfg.CueMaxSpeed-e.Cfg.CueMinSpeed)*shot.Power
	dir := Vec2{math.Cos(shot.AimAngle), math.Sin(shot.AimAngle)}
	cue.Vel = dir.Mul(speed)
	// Vertical tip offset creates draw/follow rolling-axis spin. Horizontal offset creates side spin.
	tang := dir.Perp()
	spinScale := speed / e.Table.Ball.Radius * e.Cfg.CueSpinFactor
	cue.Omega.X += -tang.X * shot.CueOffsetY * spinScale
	cue.Omega.Y += -tang.Y * shot.CueOffsetY * spinScale
	cue.Omega.Z += -shot.CueOffsetX * spinScale

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
			start := make([]Vec2, len(balls))
			for i := range balls {
				start[i] = balls[i].Pos
			}
			e.integrate(balls, sdt, simTime+float64(k+1)*sdt, &rep)
			e.solveSweptBallContacts(balls, start, sdt, simTime+sdt, &rep, &firstContactTime)
			e.solveBallContacts(balls, simTime+sdt, &rep, &firstContactTime)
			e.solveSegments(balls, simTime+sdt, &rep, railSeen, firstContactTime)
			e.checkPocketsAndBounds(balls, simTime+sdt, &rep)
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
			slip := Vec2{b.Vel.X - r*b.Omega.Y, b.Vel.Y + r*b.Omega.X}
			sl := slip.Len()
			if sl > 0.018 {
				a := slip.Mul(-e.Cfg.SlidingFriction * g / sl)
				b.Vel = b.Vel.Add(a.Mul(dt))
				// Solid sphere inertia 2/5 m r^2; angular acceleration from friction torque = 5/(2r) * a rotated.
				b.Omega.X += (5.0 / (2 * r)) * a.Y * dt
				b.Omega.Y += -(5.0 / (2 * r)) * a.X * dt
			} else {
				s := b.Vel.Len()
				if s > 0 {
					dec := e.Cfg.RollingResistance * g * dt
					ns := math.Max(0, s-dec)
					b.Vel = b.Vel.Mul(ns / s)
				}
				targetWX := -b.Vel.Y / r
				targetWY := b.Vel.X / r
				blend := math.Min(1, dt*18)
				b.Omega.X += (targetWX - b.Omega.X) * blend
				b.Omega.Y += (targetWY - b.Omega.Y) * blend
			}
			spinDecay := math.Exp(-e.Cfg.SpinDecay * dt)
			b.Omega.Z *= spinDecay
			b.Pos = b.Pos.Add(b.Vel.Mul(dt))
			b.Z = r
			if b.Vel.Len() < e.Cfg.SleepLinearSpeed && b.Omega.Len() < e.Cfg.SleepAngularSpeed {
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

func (e *Engine) solveSweptBallContacts(balls []Ball, start []Vec2, dt, eventTime float64, rep *ShotReport, firstTime *float64) {
	if dt <= 0 {
		return
	}
	r := e.Table.Ball.Radius
	m := e.Table.Ball.Mass
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
			rel := balls[j].Vel.Sub(balls[i].Vel)
			vn := rel.Dot(n)
			if vn >= 0 {
				continue
			}

			jn := -(1 + e.Cfg.BallRestitution) * vn / (2 / m)
			impulse := n.Mul(jn)
			balls[i].Vel = balls[i].Vel.Sub(impulse.Mul(1 / m))
			balls[j].Vel = balls[j].Vel.Add(impulse.Mul(1 / m))

			tangent := n.Perp()
			vt := balls[j].Vel.Sub(balls[i].Vel).Dot(tangent) - r*(balls[i].Omega.Z+balls[j].Omega.Z)
			jt := -vt / (2/m + 2.5/m)
			limit := e.Cfg.BallFriction * math.Abs(jn)
			if jt > limit {
				jt = limit
			}
			if jt < -limit {
				jt = -limit
			}
			ti := tangent.Mul(jt)
			balls[i].Vel = balls[i].Vel.Sub(ti.Mul(1 / m))
			balls[j].Vel = balls[j].Vel.Add(ti.Mul(1 / m))
			balls[i].Omega.Z -= 2.5 * jt / (m * r)
			balls[j].Omega.Z -= 2.5 * jt / (m * r)

			// Advance the remaining fraction using the post-impact velocities. A
			// subsequent overlap/segment pass resolves simultaneous contacts.
			remaining := dt - toi
			balls[i].Pos = pa.Add(balls[i].Vel.Mul(remaining))
			balls[j].Pos = pb.Add(balls[j].Vel.Mul(remaining))
			intensity := math.Min(1, math.Abs(vn)/5)
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
	m := e.Table.Ball.Mass
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
				rel := balls[j].Vel.Sub(balls[i].Vel)
				vn := rel.Dot(n)
				if vn < 0 {
					jn := -(1 + e.Cfg.BallRestitution) * vn / (2 / m)
					impulse := n.Mul(jn)
					balls[i].Vel = balls[i].Vel.Sub(impulse.Mul(1 / m))
					balls[j].Vel = balls[j].Vel.Add(impulse.Mul(1 / m))
					tangent := n.Perp()
					vt := balls[j].Vel.Sub(balls[i].Vel).Dot(tangent) - r*(balls[i].Omega.Z+balls[j].Omega.Z)
					jt := -vt / (2/m + 2.5/m)
					limit := e.Cfg.BallFriction * math.Abs(jn)
					if jt > limit {
						jt = limit
					}
					if jt < -limit {
						jt = -limit
					}
					ti := tangent.Mul(jt)
					balls[i].Vel = balls[i].Vel.Sub(ti.Mul(1 / m))
					balls[j].Vel = balls[j].Vel.Add(ti.Mul(1 / m))
					balls[i].Omega.Z -= 2.5 * jt / (m * r)
					balls[j].Omega.Z -= 2.5 * jt / (m * r)
					intensity := math.Min(1, math.Abs(vn)/5)
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
			normalImpulse := -(1 + e.Cfg.CushionRestitution) * vn
			b.Vel = b.Vel.Add(n.Mul(normalImpulse))
			tangent := n.Perp()
			vt := b.Vel.Dot(tangent) - b.Omega.Z*r
			deltaT := -vt * e.Cfg.CushionFriction
			b.Vel = b.Vel.Add(tangent.Mul(deltaT))
			b.Omega.Z -= 2.5 * deltaT / r
			b.Pos = b.Pos.Add(n.Mul(r - dist + 0.00001))
			intensity := math.Min(1, math.Abs(vn)/5)
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
