package match

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"poolarena/game-server/internal/config"
	"poolarena/game-server/internal/physics"
	"poolarena/game-server/internal/rules"
	"time"
)

const (
	StateAwaitingShot = "TURN_AWAITING_SHOT"
	StateBallsMoving  = "BALLS_MOVING"
	StateBallInHand   = "BALL_IN_HAND"
	StateBreakOption  = "BREAK_OPTION"
	StateFinished     = "MATCH_FINISHED"
	StatePaused       = "PAUSED"
)

type Player struct {
	Principal string `json:"-"`
	Nickname  string `json:"nickname"`
	CueSkin   string `json:"cueSkin"`
}
type ShotInput struct {
	RequestID                               string `json:"requestId"`
	TurnNonce                               string `json:"turnNonce"`
	AimAngle, Power, CueOffsetX, CueOffsetY float64
	CalledBall, CalledPocket                int
	Safety                                  bool
}
type PendingShot struct {
	Shooter    int
	Input      ShotInput
	Before     []physics.Ball
	Simulation physics.Simulation
	Outcome    rules.Outcome
	StartedAt  time.Time
}

type Game struct {
	ID                 string            `json:"id"`
	LobbyID            string            `json:"lobbyId"`
	Players            [2]Player         `json:"players"`
	Balls              []physics.Ball    `json:"balls"`
	Turn               int               `json:"turn"`
	TurnNonce          string            `json:"turnNonce"`
	State              string            `json:"state"`
	RuleState          rules.State       `json:"ruleState"`
	BallInHand         bool              `json:"ballInHand"`
	BallInHandHeadOnly bool              `json:"ballInHandHeadOnly"`
	ShotNumber         int               `json:"shotNumber"`
	StartedAt          time.Time         `json:"startedAt"`
	FinishedAt         time.Time         `json:"finishedAt,omitempty"`
	Winner             int               `json:"winner"`
	Loser              int               `json:"loser"`
	EndReason          string            `json:"endReason,omitempty"`
	Fouls              [2]int            `json:"fouls"`
	Pocketed           [2]int            `json:"pocketedCounts"`
	RackSeed           int64             `json:"rackSeed"`
	Pending            *PendingShot      `json:"-"`
	PendingChoice      rules.BreakChoice `json:"pendingChoice"`
	engine             *physics.Engine
	cfg                config.All
}

func New(lobbyID string, players [2]Player, breaker int, cfg config.All, engine *physics.Engine) *Game {
	seed := time.Now().UnixNano()
	g := &Game{ID: newID(), LobbyID: lobbyID, Players: players, Turn: breaker, State: StateAwaitingShot, Winner: -1, Loser: -1, RuleState: rules.State{Groups: [2]rules.Group{rules.Open, rules.Open}, Break: true}, StartedAt: time.Now().UTC(), RackSeed: seed, engine: engine, cfg: cfg}
	g.Balls = physics.NewRack(cfg.Table, seed)
	g.newTurnNonce()
	return g
}

func (g *Game) StartShot(shooter int, in ShotInput) (physics.Simulation, rules.Outcome, error) {
	if g.State != StateAwaitingShot {
		return physics.Simulation{}, rules.Outcome{}, errors.New("match_not_accepting_shot")
	}
	if shooter != g.Turn {
		return physics.Simulation{}, rules.Outcome{}, errors.New("not_your_turn")
	}
	if in.TurnNonce == "" || in.TurnNonce != g.TurnNonce {
		return physics.Simulation{}, rules.Outcome{}, errors.New("stale_turn_nonce")
	}
	if !finite(in.Power) || !finite(in.CueOffsetX) || !finite(in.CueOffsetY) || in.Power < 0.02 || in.Power > 1 || in.CueOffsetX*in.CueOffsetX+in.CueOffsetY*in.CueOffsetY > 1 {
		return physics.Simulation{}, rules.Outcome{}, errors.New("invalid_shot_values")
	}
	if !finite(in.AimAngle) {
		return physics.Simulation{}, rules.Outcome{}, errors.New("invalid_aim")
	}
	if in.CalledBall < 0 || in.CalledBall > 15 || in.CalledPocket < -1 || in.CalledPocket > 5 {
		return physics.Simulation{}, rules.Outcome{}, errors.New("invalid_call")
	}
	before := clone(g.Balls)
	sim, err := g.engine.SimulateShot(before, physics.ShotRequest{AimAngle: in.AimAngle, Power: in.Power, CueOffsetX: in.CueOffsetX, CueOffsetY: in.CueOffsetY})
	if err != nil {
		return physics.Simulation{}, rules.Outcome{}, err
	}
	outcome := rules.Evaluate(g.RuleState, shooter, rules.ShotCall{CalledBall: in.CalledBall, CalledPocket: in.CalledPocket, Safety: in.Safety}, before, sim.Final, sim.Report)
	g.Pending = &PendingShot{Shooter: shooter, Input: in, Before: before, Simulation: sim, Outcome: outcome, StartedAt: time.Now().UTC()}
	g.State = StateBallsMoving
	return sim, outcome, nil
}

func (g *Game) FinishShot() (rules.Outcome, error) {
	if g.Pending == nil || g.State != StateBallsMoving {
		return rules.Outcome{}, errors.New("no_pending_shot")
	}
	p := g.Pending
	out := p.Outcome
	g.Balls = clone(p.Simulation.Final)
	g.ShotNumber++
	for _, id := range p.Simulation.Report.Pocketed {
		if id != 0 {
			g.Pocketed[p.Shooter]++
		}
	}
	if out.Foul {
		g.Fouls[p.Shooter]++
	}
	g.Pending = nil
	if out.Winner >= 0 {
		g.finish(out.Winner, out.Loser, "eight_ball")
		return out, nil
	}
	if out.BreakChoice.Required {
		g.PendingChoice = out.BreakChoice
		g.State = StateBreakOption
		return out, nil
	}
	if g.RuleState.Break {
		g.RuleState.Break = false
	}
	g.RuleState.Groups = out.Groups
	g.Turn = out.NextPlayer
	if out.BallInHand {
		g.prepareBallInHand(out.BallInHandHeadOnly)
	} else {
		g.State = StateAwaitingShot
		g.newTurnNonce()
	}
	if out.Continue {
		g.Turn = p.Shooter
		g.State = StateAwaitingShot
		g.newTurnNonce()
	}
	return out, nil
}

func (g *Game) ResolveBreakChoice(actor int, choice string) error {
	if g.State != StateBreakOption || !g.PendingChoice.Required {
		return errors.New("no_break_choice")
	}
	if actor != g.PendingChoice.Actor {
		return errors.New("not_choice_actor")
	}
	allowed := false
	for _, x := range g.PendingChoice.Options {
		if x == choice {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.New("invalid_break_choice")
	}
	originalShooter := 1 - actor
	if g.PendingChoice.Kind == "eight_legal" {
		originalShooter = actor
	}
	switch choice {
	case "spot_8_continue":
		g.spotEight()
		g.RuleState.Break = false
		g.Turn = actor
		g.State = StateAwaitingShot
		g.newTurnNonce()
	case "rerack_self_break":
		g.rerack(actor)
	case "spot_8_bih_head":
		g.spotEight()
		g.RuleState.Break = false
		g.Turn = actor
		g.prepareBallInHand(true)
	case "rerack_opponent_break":
		g.rerack(actor)
	case "rerack_shooter_break":
		g.rerack(originalShooter)
	case "accept_table":
		g.RuleState.Break = false
		g.Turn = actor
		if cue := ballByID(g.Balls, 0); cue == nil || cue.State != physics.BallOnTable {
			g.prepareBallInHand(true)
		} else {
			g.State = StateAwaitingShot
			g.newTurnNonce()
		}
	case "ball_in_hand_head":
		g.RuleState.Break = false
		g.Turn = actor
		g.prepareBallInHand(true)
	default:
		return errors.New("invalid_break_choice")
	}
	g.PendingChoice = rules.BreakChoice{}
	return nil
}

func (g *Game) PlaceCueBall(player int, p physics.Vec2) error {
	if g.State != StateBallInHand || player != g.Turn {
		return errors.New("ball_in_hand_not_available")
	}
	if g.BallInHandHeadOnly && p.X > g.cfg.Table.Rack.HeadStringX-g.cfg.Table.Ball.Radius {
		return errors.New("must_place_above_head_string")
	}
	if !g.engine.Geometry.LegalPlacement(p, g.Balls, 0) {
		return errors.New("illegal_cue_position")
	}
	cue := ballByID(g.Balls, 0)
	if cue == nil {
		return errors.New("cue_missing")
	}
	cue.Pos = p
	cue.Z = g.cfg.Table.Ball.Radius
	cue.Vel = physics.Vec2{}
	cue.Omega = physics.Vec3{}
	cue.State = physics.BallOnTable
	cue.PocketID = -1
	cue.VZ = 0
	g.BallInHand = false
	g.BallInHandHeadOnly = false
	g.State = StateAwaitingShot
	g.newTurnNonce()
	return nil
}

func (g *Game) ShotClockFoul() {
	if g.State != StateAwaitingShot && g.State != StateBallInHand {
		return
	}
	shooter := g.Turn
	g.Fouls[shooter]++
	g.Turn = 1 - shooter
	g.RuleState.Break = false
	g.prepareBallInHand(false)
}

func (g *Game) Forfeit(loser int, reason string) {
	if g.State == StateFinished {
		return
	}
	g.finish(1-loser, loser, reason)
}
func (g *Game) Pause() {
	if g.State != StateFinished {
		g.State = StatePaused
	}
}
func (g *Game) Resume(previous string) {
	if g.State == StatePaused {
		g.State = previous
	}
}

func (g *Game) prepareBallInHand(headOnly bool) {
	cue := ballByID(g.Balls, 0)
	if cue != nil {
		cue.State = physics.BallPocketed
		cue.Vel = physics.Vec2{}
		cue.Omega = physics.Vec3{}
	}
	g.BallInHand = true
	g.BallInHandHeadOnly = headOnly
	g.State = StateBallInHand
	g.newTurnNonce()
}
func (g *Game) rerack(breaker int) {
	g.RackSeed++
	g.Balls = physics.NewRack(g.cfg.Table, g.RackSeed)
	g.RuleState = rules.State{Groups: [2]rules.Group{rules.Open, rules.Open}, Break: true}
	g.Turn = breaker
	g.BallInHand = false
	g.BallInHandHeadOnly = false
	g.State = StateAwaitingShot
	g.newTurnNonce()
}
func (g *Game) spotEight() {
	b := ballByID(g.Balls, 8)
	if b == nil {
		return
	}
	b.State = physics.BallOnTable
	b.Z = g.cfg.Table.Ball.Radius
	b.Vel = physics.Vec2{}
	b.Omega = physics.Vec3{}
	b.PocketID = -1
	geom := g.engine.Geometry
	step := 2*geom.Radius + 0.0003
	for i := 0; i < 24; i++ {
		p := physics.Vec2{X: geom.HalfL/2 + float64(i)*step, Y: 0}
		if p.X > geom.HalfL-geom.Radius {
			p.X = geom.HalfL/2 - float64(i)*step
		}
		if geom.LegalPlacement(p, g.Balls, 8) {
			b.Pos = p
			return
		}
	}
}
func (g *Game) finish(winner, loser int, reason string) {
	g.Winner = winner
	g.Loser = loser
	g.EndReason = reason
	g.State = StateFinished
	g.FinishedAt = time.Now().UTC()
	g.BallInHand = false
}
func (g *Game) newTurnNonce() { g.TurnNonce = randomHex(16) }
func finite(v float64) bool   { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func clone(in []physics.Ball) []physics.Ball {
	out := make([]physics.Ball, len(in))
	copy(out, in)
	return out
}
func ballByID(b []physics.Ball, id int) *physics.Ball {
	for i := range b {
		if b[i].ID == id {
			return &b[i]
		}
	}
	return nil
}
func newID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}
func randomHex(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
