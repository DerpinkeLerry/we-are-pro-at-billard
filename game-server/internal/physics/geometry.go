package physics

import (
	"math"
	"poolarena/game-server/internal/config"
)

type Segment struct {
	A, B Vec2
	Kind string
	ID   int
}
type Pocket struct {
	ID                                                        int
	Kind                                                      string
	MouthMid, ThroatMid, Dir, Tangent                         Vec2
	MouthWidth, ThroatWidth, Shelf, DropDepth, DropRX, DropRY float64
	BackDraftDeg                                              float64
}
type TableGeometry struct {
	HalfL, HalfW, Radius float64
	Segments             []Segment
	Pockets              []Pocket
}

func BuildGeometry(c config.Table) TableGeometry {
	g := TableGeometry{HalfL: c.PlayingSurface.Length / 2, HalfW: c.PlayingSurface.Width / 2, Radius: c.Ball.Radius}
	cm := c.Pockets.Corner.Mouth / math.Sqrt2
	sm := c.Pockets.Side.Mouth / 2
	id := 0
	add := func(a, b Vec2, kind string) {
		g.Segments = append(g.Segments, Segment{A: a, B: b, Kind: kind, ID: id})
		id++
	}
	// Long rails, split around the side pockets and corner mouths.
	add(Vec2{-g.HalfL + cm, g.HalfW}, Vec2{-sm, g.HalfW}, "cushion")
	add(Vec2{sm, g.HalfW}, Vec2{g.HalfL - cm, g.HalfW}, "cushion")
	add(Vec2{-g.HalfL + cm, -g.HalfW}, Vec2{-sm, -g.HalfW}, "cushion")
	add(Vec2{sm, -g.HalfW}, Vec2{g.HalfL - cm, -g.HalfW}, "cushion")
	// Short rails.
	add(Vec2{-g.HalfL, -g.HalfW + cm}, Vec2{-g.HalfL, g.HalfW - cm}, "cushion")
	add(Vec2{g.HalfL, -g.HalfW + cm}, Vec2{g.HalfL, g.HalfW - cm}, "cushion")

	pid := 0
	for _, sx := range []float64{-1, 1} {
		for _, sy := range []float64{-1, 1} {
			pc := c.Pockets.Corner
			throatWidth := pc.ThroatWidth
			d := Vec2{sx / math.Sqrt2, sy / math.Sqrt2}
			t := Vec2{sy / math.Sqrt2, -sx / math.Sqrt2}
			corner := Vec2{sx * g.HalfL, sy * g.HalfW}
			mouthMid := corner.Sub(d.Mul(pc.Mouth / 2))
			throatMid := mouthMid.Add(d.Mul(pc.Shelf))
			ma := mouthMid.Add(t.Mul(pc.Mouth / 2))
			mb := mouthMid.Sub(t.Mul(pc.Mouth / 2))
			ta := throatMid.Add(t.Mul(throatWidth / 2))
			tb := throatMid.Sub(t.Mul(throatWidth / 2))
			add(ma, ta, "jaw")
			add(mb, tb, "jaw")
			g.Pockets = append(g.Pockets, Pocket{ID: pid, Kind: "corner", MouthMid: mouthMid, ThroatMid: throatMid, Dir: d, Tangent: t, MouthWidth: pc.Mouth, ThroatWidth: throatWidth, Shelf: pc.Shelf, DropDepth: pc.DropDepth, DropRX: pc.DropRadiusX, DropRY: pc.DropRadiusY, BackDraftDeg: pc.BackDraftDeg})
			pid++
		}
	}
	for _, sy := range []float64{-1, 1} {
		pc := c.Pockets.Side
		throatWidth := pc.ThroatWidth
		d := Vec2{0, sy}
		t := Vec2{1, 0}
		mouthMid := Vec2{0, sy * g.HalfW}
		throatMid := mouthMid.Add(d.Mul(pc.Shelf))
		ma := mouthMid.Add(t.Mul(pc.Mouth / 2))
		mb := mouthMid.Sub(t.Mul(pc.Mouth / 2))
		ta := throatMid.Add(t.Mul(throatWidth / 2))
		tb := throatMid.Sub(t.Mul(throatWidth / 2))
		add(ma, ta, "jaw")
		add(mb, tb, "jaw")
		g.Pockets = append(g.Pockets, Pocket{ID: pid, Kind: "side", MouthMid: mouthMid, ThroatMid: throatMid, Dir: d, Tangent: t, MouthWidth: pc.Mouth, ThroatWidth: throatWidth, Shelf: pc.Shelf, DropDepth: pc.DropDepth, DropRX: pc.DropRadiusX, DropRY: pc.DropRadiusY, BackDraftDeg: pc.BackDraftDeg})
		pid++
	}
	return g
}

func closestPointSegment(p, a, b Vec2) Vec2 {
	ab := b.Sub(a)
	den := ab.Len2()
	if den < 1e-14 {
		return a
	}
	t := p.Sub(a).Dot(ab) / den
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return a.Add(ab.Mul(t))
}

func (g TableGeometry) pocketForFalling(p Vec2) (Pocket, bool) {
	for _, pk := range g.Pockets {
		rel := p.Sub(pk.ThroatMid)
		depth := rel.Dot(pk.Dir)
		lat := rel.Dot(pk.Tangent)
		if depth >= 0 && math.Abs(lat) <= pk.ThroatWidth/2-g.Radius*0.08 {
			return pk, true
		}
	}
	return Pocket{}, false
}

func (g TableGeometry) PocketByID(id int) (Pocket, bool) {
	for _, pk := range g.Pockets {
		if pk.ID == id {
			return pk, true
		}
	}
	return Pocket{}, false
}

func (g TableGeometry) InPlayableArea(p Vec2) bool {
	return math.Abs(p.X) <= g.HalfL-g.Radius && math.Abs(p.Y) <= g.HalfW-g.Radius
}

func (g TableGeometry) LegalPlacement(p Vec2, balls []Ball, ignoreID int) bool {
	if !g.InPlayableArea(p) {
		return false
	}
	// Reject positions close to a pocket mouth where the cloth does not support a full ball.
	for _, pk := range g.Pockets {
		rel := p.Sub(pk.MouthMid)
		if rel.Dot(pk.Dir) > -g.Radius*0.35 && math.Abs(rel.Dot(pk.Tangent)) < pk.MouthWidth/2+g.Radius*0.25 {
			return false
		}
	}
	min := 2*g.Radius + 0.0002
	min2 := min * min
	for _, b := range balls {
		if b.ID == ignoreID || b.State != BallOnTable {
			continue
		}
		if p.Sub(b.Pos).Len2() < min2 {
			return false
		}
	}
	return true
}
