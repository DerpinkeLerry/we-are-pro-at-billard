package physics

import "math"

type Vec2 struct{ X, Y float64 }

func (a Vec2) Add(b Vec2) Vec2    { return Vec2{a.X + b.X, a.Y + b.Y} }
func (a Vec2) Sub(b Vec2) Vec2    { return Vec2{a.X - b.X, a.Y - b.Y} }
func (a Vec2) Mul(s float64) Vec2 { return Vec2{a.X * s, a.Y * s} }
func (a Vec2) Dot(b Vec2) float64 { return a.X*b.X + a.Y*b.Y }
func (a Vec2) Len2() float64      { return a.Dot(a) }
func (a Vec2) Len() float64       { return math.Sqrt(a.Len2()) }
func (a Vec2) Normalized() Vec2 {
	l := a.Len()
	if l < 1e-12 {
		return Vec2{}
	}
	return a.Mul(1 / l)
}
func (a Vec2) Perp() Vec2 { return Vec2{-a.Y, a.X} }

type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Len() float64 { return math.Sqrt(a.X*a.X + a.Y*a.Y + a.Z*a.Z) }

type BallState string

const (
	BallOnTable  BallState = "table"
	BallFalling  BallState = "falling"
	BallPocketed BallState = "pocketed"
	BallOffTable BallState = "off_table"
)

type Ball struct {
	ID       int       `json:"id"`
	Pos      Vec2      `json:"pos"`
	Vel      Vec2      `json:"vel"`
	Omega    Vec3      `json:"omega"`
	Z        float64   `json:"z"`
	VZ       float64   `json:"vz"`
	State    BallState `json:"state"`
	PocketID int       `json:"pocketId"`
	SleepFor float64   `json:"-"`
}

type BallSnapshot struct {
	ID       int       `json:"id"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	Z        float64   `json:"z"`
	VX       float64   `json:"vx"`
	VY       float64   `json:"vy"`
	WX       float64   `json:"wx"`
	WY       float64   `json:"wy"`
	WZ       float64   `json:"wz"`
	State    BallState `json:"state"`
	PocketID int       `json:"pocketId"`
}

type Frame struct {
	Time  float64        `json:"time"`
	Balls []BallSnapshot `json:"balls"`
}

type Event struct {
	Time      float64 `json:"time"`
	Type      string  `json:"type"`
	A         int     `json:"a,omitempty"`
	B         int     `json:"b,omitempty"`
	PocketID  int     `json:"pocketId,omitempty"`
	Intensity float64 `json:"intensity"`
}

type ShotRequest struct {
	AimAngle   float64 `json:"aimAngle"`
	Power      float64 `json:"power"`
	CueOffsetX float64 `json:"cueOffsetX"`
	CueOffsetY float64 `json:"cueOffsetY"`
}

type ShotReport struct {
	FirstObjectBall    int         `json:"firstObjectBall"`
	Pocketed           []int       `json:"pocketed"`
	PocketByBall       map[int]int `json:"pocketByBall"`
	CueScratch         bool        `json:"cueScratch"`
	BallsOffTable      []int       `json:"ballsOffTable"`
	RailBallIDs        []int       `json:"railBallIds"`
	AnyRailAfterFirst  bool        `json:"anyRailAfterFirst"`
	SimulationDuration float64     `json:"simulationDuration"`
	Events             []Event     `json:"events"`
}

type Simulation struct {
	Frames []Frame
	Final  []Ball
	Report ShotReport
}

func SnapshotBalls(balls []Ball) []BallSnapshot {
	out := make([]BallSnapshot, 0, len(balls))
	for _, b := range balls {
		out = append(out, BallSnapshot{ID: b.ID, X: b.Pos.X, Y: b.Pos.Y, Z: b.Z, VX: b.Vel.X, VY: b.Vel.Y, WX: b.Omega.X, WY: b.Omega.Y, WZ: b.Omega.Z, State: b.State, PocketID: b.PocketID})
	}
	return out
}
