package physics

import "math"

// SweptCircleTOI returns the first time t in [0,maxT] at which two moving circles touch.
func SweptCircleTOI(pa, va, pb, vb Vec2, radius, maxT float64) (float64, bool) {
	p := pa.Sub(pb)
	v := va.Sub(vb)
	rr := 2 * radius
	c := p.Len2() - rr*rr
	if c <= 0 {
		return 0, true
	}
	a := v.Len2()
	if a < 1e-14 {
		return 0, false
	}
	b := 2 * p.Dot(v)
	disc := b*b - 4*a*c
	if disc < 0 {
		return 0, false
	}
	root := math.Sqrt(disc)
	t := (-b - root) / (2 * a)
	if t < 0 || t > maxT {
		return 0, false
	}
	return t, true
}
