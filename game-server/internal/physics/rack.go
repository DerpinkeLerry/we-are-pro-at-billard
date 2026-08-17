package physics

import (
	"math"
	"math/rand"
	"poolarena/game-server/internal/config"
)

func NewRack(table config.Table, seed int64) []Ball {
	r := table.Ball.Radius
	balls := make([]Ball, 16)
	for i := 0; i < 16; i++ {
		balls[i] = Ball{ID: i, Z: r, State: BallOnTable, PocketID: -1}
	}
	balls[0].Pos = Vec2{table.Rack.CueBreakX, 0}
	solids := []int{1, 2, 3, 4, 5, 6, 7}
	stripes := []int{9, 10, 11, 12, 13, 14, 15}
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(solids), func(i, j int) { solids[i], solids[j] = solids[j], solids[i] })
	rng.Shuffle(len(stripes), func(i, j int) { stripes[i], stripes[j] = stripes[j], stripes[i] })
	slots := make([]int, 15)
	for i := range slots {
		slots[i] = -1
	}
	slots[4] = 8 // row 3 center in flattened rows: 0 | 1,2 | 3,4,5
	slots[10] = solids[0]
	slots[14] = stripes[0] // rear corners are opposite groups
	pool := append(append([]int{}, solids[1:]...), stripes[1:]...)
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	pi := 0
	for i := range slots {
		if slots[i] < 0 {
			slots[i] = pool[pi]
			pi++
		}
	}
	spacing := 2*r + 0.00008
	dx := spacing * math.Sqrt(3) / 2
	idx := 0
	for row := 0; row < 5; row++ {
		x := table.Rack.FootSpotX + float64(row)*dx
		for col := 0; col <= row; col++ {
			y := (float64(col) - float64(row)/2) * spacing
			id := slots[idx]
			balls[id].Pos = Vec2{x, y}
			idx++
		}
	}
	return balls
}

func ResetCueBall(balls []Ball, table config.Table, headOnly bool) bool {
	cue := findBall(balls, 0)
	if cue == nil {
		return false
	}
	cue.State = BallOnTable
	cue.PocketID = -1
	cue.Vel = Vec2{}
	cue.Omega = Vec3{}
	cue.Z = table.Ball.Radius
	cue.VZ = 0
	candidates := []Vec2{{table.Rack.CueBreakX, 0}, {-0.9, 0.15}, {-0.9, -0.15}, {-0.7, 0.25}, {-0.7, -0.25}}
	g := BuildGeometry(table)
	for _, p := range candidates {
		if headOnly && p.X > table.Rack.HeadStringX-table.Ball.Radius {
			continue
		}
		if g.LegalPlacement(p, balls, 0) {
			cue.Pos = p
			return true
		}
	}
	return false
}
