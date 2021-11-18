// Copyright 2017 by the rasterx Authors. All rights reserved.
//
// created: 2017 by S.R.Wiley

package rasterx

import (
	"math"

	"golang.org/x/image/math/fixed"
)

const (
	cubicsPerHalfCircle = 8                 // Number of cubic beziers to approx half a circle
	epsilonFixed        = fixed.Int26_6(16) // 1/4 in fixed point
	// fixed point t paramaterization shift factor;
	// (2^this)/64 is the max length of t for fixed.Int26_6
	tStrokeShift = 14
)

type (
	// JoinMode type to specify how segments join.
	JoinMode uint8
	// CapFunc defines a function that draws caps on the ends of lines
	CapFunc func(p Adder, a, eNorm fixed.Point26_6)
	// GapFunc defines a function to bridge gaps when the miter limit is
	// exceeded
	GapFunc func(p Adder, a, tNorm, lNorm fixed.Point26_6)

	// C2Point represents a point that connects two stroke segments
	// and holds the tangent, normal and radius of curvature
	// of the trailing and leading segments in fixed point values.
	C2Point struct {
		P, TTan, LTan, TNorm, LNorm fixed.Point26_6
		RT, RL                      fixed.Int26_6
	}

	// Stroker does everything a Filler does, but
	// also allows for stroking and dashed stroking in addition to
	// filling
	Stroker struct {
		Filler
		CapT, CapL CapFunc // Trailing and leading cap funcs may be set separately
		JoinGap    GapFunc // When gap appears between segments, this function is called

		firstP, trailPoint, leadPoint C2Point         // Tracks progress of the stroke
		ln                            fixed.Point26_6 // last normal of intra-seg connection.
		u, mLimit                     fixed.Int26_6   // u is the half-width of the stroke.

		JoinMode JoinMode
		inStroke bool
	}
)

// JoinMode constants determine how stroke segments bridge the gap at a join
// ArcClip mode is like MiterClip applied to arcs, and is not part of the SVG2.0
// standard.
const (
	Arc JoinMode = iota
	ArcClip
	Miter
	MiterClip
	Bevel
	Round
)

// NewStroker returns a ptr to a Stroker with default values.
// A Stroker has all of the capabilities of a Filler and Scanner, plus the ability
// to stroke curves with solid lines. Use SetStroke to configure with non-default
// values.
func NewStroker(width, height int, scanner Scanner) *Stroker {
	r := new(Stroker)
	r.Scanner = scanner
	r.SetBounds(width, height)
	//Defaults for stroking
	r.SetWinding(true)
	r.u = 2 << 6
	r.mLimit = 4 << 6
	r.JoinMode = MiterClip
	r.JoinGap = RoundGap
	r.CapL = RoundCap
	r.CapT = RoundCap
	r.SetStroke(1<<6, 4<<6, ButtCap, nil, FlatGap, MiterClip)
	return r
}

// SetStroke set the parameters for stroking a line. width is the width of the line, miterlimit is the miter cutoff
// value for miter, arc, miterclip and arcClip joinModes. CapL and CapT are the capping functions for leading and trailing
// line ends. If one is nil, the other function is used at both ends. If both are nil, both ends are ButtCapped.
// gp is the gap function that determines how a gap on the convex side of two joining lines is filled. jm is the JoinMode
// for curve segments.
func (r *Stroker) SetStroke(width, miterLimit fixed.Int26_6, capL, capT CapFunc, gp GapFunc, jm JoinMode) {
	r.u = width / 2
	r.CapL = capL
	r.CapT = capT
	r.JoinMode = jm
	r.JoinGap = gp
	r.mLimit = (r.u * miterLimit) >> 6

	if r.CapT == nil {
		if r.CapL == nil {
			r.CapT = ButtCap
		} else {
			r.CapT = r.CapL
		}
	}
	if r.CapL == nil {
		r.CapL = r.CapT
	}
	if gp == nil {
		if r.JoinMode == Round {
			r.JoinGap = RoundGap
		} else {
			r.JoinGap = FlatGap
		}
	}

}

// GapToCap is a utility that converts a CapFunc to GapFunc
func GapToCap(p Adder, a, eNorm fixed.Point26_6, gf GapFunc) {
	p.Start(a.Add(eNorm))
	gf(p, a, eNorm, Invert(eNorm))
	p.Line(a.Sub(eNorm))
}

var (
	// ButtCap caps lines with a straight line
	ButtCap CapFunc = func(p Adder, a, eNorm fixed.Point26_6) {
		p.Start(a.Add(eNorm))
		p.Line(a.Sub(eNorm))
	}
	// SquareCap caps lines with a square which is slightly longer than ButtCap
	SquareCap CapFunc = func(p Adder, a, eNorm fixed.Point26_6) {
		tpt := a.Add(turnStarboard90(eNorm))
		p.Start(a.Add(eNorm))
		p.Line(tpt.Add(eNorm))
		p.Line(tpt.Sub(eNorm))
		p.Line(a.Sub(eNorm))
	}
	// RoundCap caps lines with a half-circle
	RoundCap CapFunc = func(p Adder, a, eNorm fixed.Point26_6) {
		GapToCap(p, a, eNorm, RoundGap)
	}
	// CubicCap caps lines with a cubic bezier
	CubicCap CapFunc = func(p Adder, a, eNorm fixed.Point26_6) {
		GapToCap(p, a, eNorm, CubicGap)
	}
	// QuadraticCap caps lines with a quadratic bezier
	QuadraticCap CapFunc = func(p Adder, a, eNorm fixed.Point26_6) {
		GapToCap(p, a, eNorm, QuadraticGap)
	}
	// Gap functions

	//FlatGap bridges miter-limit gaps with a straight line
	FlatGap GapFunc = func(p Adder, a, tNorm, lNorm fixed.Point26_6) {
		p.Line(a.Add(lNorm))
	}
	// RoundGap bridges miter-limit gaps with a circular arc
	RoundGap GapFunc = func(p Adder, a, tNorm, lNorm fixed.Point26_6) {
		strokeArc(p, a, a.Add(tNorm), a.Add(lNorm), true, 0, 0, p.Line)
		p.Line(a.Add(lNorm)) // just to be sure line joins cleanly,
		// last pt in stoke arc may not be precisely s2
	}
	// CubicGap bridges miter-limit gaps with a cubic bezier
	CubicGap GapFunc = func(p Adder, a, tNorm, lNorm fixed.Point26_6) {
		p.CubeBezier(a.Add(tNorm).Add(turnStarboard90(tNorm)), a.Add(lNorm).Add(turnPort90(lNorm)), a.Add(lNorm))
	}
	// QuadraticGap bridges miter-limit gaps with a quadratic bezier
	QuadraticGap GapFunc = func(p Adder, a, tNorm, lNorm fixed.Point26_6) {
		c1, c2 := a.Add(tNorm).Add(turnStarboard90(tNorm)), a.Add(lNorm).Add(turnPort90(lNorm))
		cm := c1.Add(c2).Mul(fixed.Int26_6(1 << 5))
		p.QuadBezier(cm, a.Add(lNorm))
	}
)

// StrokeArc strokes a circular arc by approximation with bezier curves
func strokeArc(p Adder, a, s1, s2 fixed.Point26_6, clockwise bool, trimStart,
	trimEnd fixed.Int26_6, firstPoint func(p fixed.Point26_6)) (ps1, ds1, ps2, ds2 fixed.Point26_6) {
	// Approximate the circular arc using a set of cubic bezier curves by the method of
	// L. Maisonobe, "Drawing an elliptical arc using polylines, quadratic
	// or cubic Bezier curves", 2003
	// https://www.spaceroots.org/documents/elllipse/elliptical-arc.pdf
	// The method was simplified for circles.
	theta1 := math.Atan2(float64(s1.Y-a.Y), float64(s1.X-a.X))
	theta2 := math.Atan2(float64(s2.Y-a.Y), float64(s2.X-a.X))
	if !clockwise {
		for theta1 < theta2 {
			theta1 += math.Pi * 2
		}
	} else {
		for theta2 < theta1 {
			theta2 += math.Pi * 2
		}
	}
	deltaTheta := theta2 - theta1
	if trimStart > 0 {
		ds := (deltaTheta * float64(trimStart)) / float64(1<<tStrokeShift)
		deltaTheta -= ds
		theta1 += ds
	}
	if trimEnd > 0 {
		ds := (deltaTheta * float64(trimEnd)) / float64(1<<tStrokeShift)
		deltaTheta -= ds
	}

	segs := int(math.Abs(deltaTheta)/(math.Pi/cubicsPerHalfCircle)) + 1
	dTheta := deltaTheta / float64(segs)
	tde := math.Tan(dTheta / 2)
	alpha := fixed.Int26_6(math.Sin(dTheta) * (math.Sqrt(4+3*tde*tde) - 1) * (64.0 / 3.0)) // Math is fun!
	r := float64(Length(s1.Sub(a)))                                                        // Note r is *64
	ldp := fixed.Point26_6{X: -fixed.Int26_6(r * math.Sin(theta1)), Y: fixed.Int26_6(r * math.Cos(theta1))}
	ds1 = ldp
	ps1 = fixed.Point26_6{X: a.X + ldp.Y, Y: a.Y - ldp.X}
	firstPoint(ps1)
	s1 = ps1
	for i := 1; i <= segs; i++ {
		eta := theta1 + dTheta*float64(i)
		ds2 = fixed.Point26_6{X: -fixed.Int26_6(r * math.Sin(eta)), Y: fixed.Int26_6(r * math.Cos(eta))}
		ps2 = fixed.Point26_6{X: a.X + ds2.Y, Y: a.Y - ds2.X} // Using deriviative to calc new pt, because circle
		p1 := s1.Add(ldp.Mul(alpha))
		p2 := ps2.Sub(ds2.Mul(alpha))
		p.CubeBezier(p1, p2, ps2)
		s1, ldp = ps2, ds2
	}
	return
}

// Joiner is called when two segments of a stroke are joined. it is exposed
// so that if can be wrapped to generate callbacks for the join points.
func (r *Stroker) Joiner(p C2Point) {
	crossProd := p.LNorm.X*p.TNorm.Y - p.TNorm.X*p.LNorm.Y
	// stroke bottom edge, with the reverse of p
	r.strokeEdge(C2Point{P: p.P, TNorm: Invert(p.LNorm), LNorm: Invert(p.TNorm),
		TTan: Invert(p.LTan), LTan: Invert(p.TTan), RT: -p.RL, RL: -p.RT}, -crossProd)
	// stroke top edge
	r.strokeEdge(p, crossProd)
}

// strokeEdge reduces code redundancy in the Joiner function by 2x since it handles
// the top and bottom edges. This function encodes most of the logic of how to
// handle joins between the given C2Point point p, and the end of the line.
func (r *Stroker) strokeEdge(p C2Point, crossProd fixed.Int26_6) {
	ra := &r.Filler
	s1, s2 := p.P.Add(p.TNorm), p.P.Add(p.LNorm) // Bevel points for top leading and trailing
	ra.Start(s1)
	if crossProd > -epsilonFixed*epsilonFixed { // Almost co-linear or convex
		ra.Line(s2)
		return // No need to fill any gaps
	}

	var ct, cl fixed.Point26_6 // Center of curvature trailing, leading
	var rt, rl fixed.Int26_6   // Radius of curvature trailing, leading

	// Adjust radiuses for stroke width
	if r.JoinMode == Arc || r.JoinMode == ArcClip {
		// Find centers of radius of curvature and adjust the radius to be drawn
		// by half the stroke width.
		if p.RT != 0 {
			if p.RT > 0 {
				ct = p.P.Add(ToLength(turnPort90(p.TTan), p.RT))
				rt = p.RT - r.u
			} else {
				ct = p.P.Sub(ToLength(turnPort90(p.TTan), -p.RT))
				rt = -p.RT + r.u
			}
			if rt < 0 {
				rt = 0
			}
		}
		if p.RL != 0 {
			if p.RL > 0 {
				cl = p.P.Add(ToLength(turnPort90(p.LTan), p.RL))
				rl = p.RL - r.u
			} else {
				cl = p.P.Sub(ToLength(turnPort90(p.LTan), -p.RL))
				rl = -p.RL + r.u
			}
			if rl < 0 {
				rl = 0
			}
		}
	}

	if r.JoinMode == MiterClip || r.JoinMode == Miter ||
		// Arc or ArcClip with 0 tRadCurve and 0 lRadCurve is treated the same as a
		// Miter or MiterClip join, resp.
		((r.JoinMode == Arc || r.JoinMode == ArcClip) && (rt == 0 && rl == 0)) {
		xt := CalcIntersect(s1.Sub(p.TTan), s1, s2, s2.Sub(p.LTan))
		xa := xt.Sub(p.P)
		if Length(xa) < r.mLimit { // within miter limit
			ra.Line(xt)
			ra.Line(s2)
			return
		}
		if r.JoinMode == MiterClip || (r.JoinMode == ArcClip) {
			//Projection of tNorm onto xa
			tProjP := xa.Mul(fixed.Int26_6((DotProd(xa, p.TNorm) << 6) / DotProd(xa, xa)))
			projLen := Length(tProjP)
			if r.mLimit > projLen { // the miter limit line is past the bevel point
				// t is the fraction shifted by tStrokeShift to scale the vectors from the bevel point
				// to the line intersection, so that they abbut the miter limit line.
				tiLength := Length(xa)
				sx1, sx2 := xt.Sub(s1), xt.Sub(s2)
				t := (r.mLimit - projLen) << tStrokeShift / (tiLength - projLen)
				tx := ToLength(sx1, t*Length(sx1)>>tStrokeShift)
				lx := ToLength(sx2, t*Length(sx2)>>tStrokeShift)
				vx := ToLength(xa, t*Length(xa)>>tStrokeShift)
				s1p, _, ap := s1.Add(tx), s2.Add(lx), p.P.Add(vx)
				gLen := Length(ap.Sub(s1p))
				ra.Line(s1p)
				r.JoinGap(ra, ap, ToLength(turnPort90(p.TTan), gLen), ToLength(turnPort90(p.LTan), gLen))
				ra.Line(s2)
				return
			}
		} // Fallthrough
	} else if r.JoinMode == Arc || r.JoinMode == ArcClip {
		// Test for cases of a bezier meeting line, an line meeting a bezier,
		// or a bezier meeting a bezier. (Line meeting line is handled above.)
		switch {
		case rt == 0: // rl != 0, because one must be non-zero as checked above
			xt, intersect := RayCircleIntersection(s1.Add(p.TTan), s1, cl, rl)
			if intersect {
				ray1, ray2 := xt.Sub(cl), s2.Sub(cl)
				clockwise := (ray1.X*ray2.Y > ray1.Y*ray2.X) // Sign of xprod
				if Length(p.P.Sub(xt)) < r.mLimit {          // within miter limit
					strokeArc(ra, cl, xt, s2, clockwise, 0, 0, ra.Line)
					ra.Line(s2)
					return
				}
				// Not within miter limit line
				if r.JoinMode == ArcClip { // Scale bevel points towards xt, and call gap func
					xa := xt.Sub(p.P)
					//Projection of tNorm onto xa
					tProjP := xa.Mul(fixed.Int26_6((DotProd(xa, p.TNorm) << 6) / DotProd(xa, xa)))
					projLen := Length(tProjP)
					if r.mLimit > projLen { // the miter limit line is past the bevel point
						// t is the fraction shifted by tStrokeShift to scale the line or arc from the bevel point
						// to the line intersection, so that they abbut the miter limit line.
						sx1 := xt.Sub(s1) //, xt.Sub(s2)
						t := fixed.Int26_6(1<<tStrokeShift) - ((r.mLimit - projLen) << tStrokeShift / (Length(xa) - projLen))
						tx := ToLength(sx1, t*Length(sx1)>>tStrokeShift)
						s1p := xt.Sub(tx)
						ra.Line(s1p)
						sp1, ds1, ps2, _ := strokeArc(ra, cl, xt, s2, clockwise, t, 0, ra.Start)
						ra.Start(s1p)
						// calc gap center as pt where -tnorm and line perp to midcoord
						midP := sp1.Add(s1p).Mul(fixed.Int26_6(1 << 5)) // midpoint
						midLine := turnPort90(midP.Sub(sp1))
						if midLine.X*midLine.X+midLine.Y*midLine.Y > epsilonFixed { // if midline is zero, CalcIntersect is invalid
							ap := CalcIntersect(s1p, s1p.Sub(p.TNorm), midLine.Add(midP), midP)
							gLen := Length(ap.Sub(s1p))
							if clockwise {
								ds1 = Invert(ds1)
							}
							r.JoinGap(ra, ap, ToLength(turnPort90(p.TTan), gLen), ToLength(turnStarboard90(ds1), gLen))
						}
						ra.Line(sp1)
						ra.Start(ps2)
						ra.Line(s2)
						return
					}
					//Bevel points not past miter limit: fallthrough
				}
			}
		case rl == 0: // rt != 0, because one must be non-zero as checked above
			xt, intersect := RayCircleIntersection(s2.Sub(p.LTan), s2, ct, rt)
			if intersect {
				ray1, ray2 := s1.Sub(ct), xt.Sub(ct)
				clockwise := ray1.X*ray2.Y > ray1.Y*ray2.X
				if Length(p.P.Sub(xt)) < r.mLimit { // within miter limit
					strokeArc(ra, ct, s1, xt, clockwise, 0, 0, ra.Line)
					ra.Line(s2)
					return
				}
				// Not within miter limit line
				if r.JoinMode == ArcClip { // Scale bevel points towards xt, and call gap func
					xa := xt.Sub(p.P)
					//Projection of lNorm onto xa
					lProjP := xa.Mul(fixed.Int26_6((DotProd(xa, p.LNorm) << 6) / DotProd(xa, xa)))
					projLen := Length(lProjP)
					if r.mLimit > projLen { // The miter limit line is past the bevel point,
						// t is the fraction to scale the line or arc from the bevel point
						// to the line intersection, so that they abbut the miter limit line.
						sx2 := xt.Sub(s2)
						t := fixed.Int26_6(1<<tStrokeShift) - ((r.mLimit - projLen) << tStrokeShift / (Length(xa) - projLen))
						lx := ToLength(sx2, t*Length(sx2)>>tStrokeShift)
						s2p := xt.Sub(lx)
						_, _, ps2, ds2 := strokeArc(ra, ct, s1, xt, clockwise, 0, t, ra.Line)
						// calc gap center as pt where -lnorm and line perp to midcoord
						midP := s2p.Add(ps2).Mul(fixed.Int26_6(1 << 5)) // midpoint
						midLine := turnStarboard90(midP.Sub(ps2))
						if midLine.X*midLine.X+midLine.Y*midLine.Y > epsilonFixed { // if midline is zero, CalcIntersect is invalid
							ap := CalcIntersect(midP, midLine.Add(midP), s2p, s2p.Sub(p.LNorm))
							gLen := Length(ap.Sub(ps2))
							if clockwise {
								ds2 = Invert(ds2)
							}
							r.JoinGap(ra, ap, ToLength(turnStarboard90(ds2), gLen), ToLength(turnPort90(p.LTan), gLen))
						}
						ra.Line(s2)
						return
					}
					//Bevel points not past miter limit: fallthrough
				}
			}
		default: // Both rl != 0 and rt != 0 as checked above
			xt1, xt2, gIntersect := CircleCircleIntersection(ct, cl, rt, rl)
			xt, intersect := ClosestPortside(s1, s2, xt1, xt2, gIntersect)
			if intersect {
				ray1, ray2 := s1.Sub(ct), xt.Sub(ct)
				clockwiseT := (ray1.X*ray2.Y > ray1.Y*ray2.X)
				ray1, ray2 = xt.Sub(cl), s2.Sub(cl)
				clockwiseL := ray1.X*ray2.Y > ray1.Y*ray2.X

				if Length(p.P.Sub(xt)) < r.mLimit { // within miter limit
					strokeArc(ra, ct, s1, xt, clockwiseT, 0, 0, ra.Line)
					strokeArc(ra, cl, xt, s2, clockwiseL, 0, 0, ra.Line)
					ra.Line(s2)
					return
				}

				if r.JoinMode == ArcClip { // Scale bevel points towards xt, and call gap func
					xa := xt.Sub(p.P)
					//Projection of lNorm onto xa
					lProjP := xa.Mul(fixed.Int26_6((DotProd(xa, p.LNorm) << 6) / DotProd(xa, xa)))
					projLen := Length(lProjP)
					if r.mLimit > projLen { // The miter limit line is past the bevel point,
						// t is the fraction to scale the line or arc from the bevel point
						// to the line intersection, so that they abbut the miter limit line.
						t := fixed.Int26_6(1<<tStrokeShift) - ((r.mLimit - projLen) << tStrokeShift / (Length(xa) - projLen))
						_, _, ps1, ds1 := strokeArc(ra, ct, s1, xt, clockwiseT, 0, t, r.Filler.Line)
						ps2, ds2, fs2, _ := strokeArc(ra, cl, xt, s2, clockwiseL, t, 0, ra.Start)
						midP := ps1.Add(ps2).Mul(fixed.Int26_6(1 << 5)) // midpoint
						midLine := turnStarboard90(midP.Sub(ps1))
						ra.Start(ps1)
						if midLine.X*midLine.X+midLine.Y*midLine.Y > epsilonFixed { // if midline is zero, CalcIntersect is invalid
							if clockwiseT {
								ds1 = Invert(ds1)
							}
							if clockwiseL {
								ds2 = Invert(ds2)
							}
							ap := CalcIntersect(midP, midLine.Add(midP), ps2, ps2.Sub(turnStarboard90(ds2)))
							gLen := Length(ap.Sub(ps2))
							r.JoinGap(ra, ap, ToLength(turnStarboard90(ds1), gLen), ToLength(turnStarboard90(ds2), gLen))
						}
						ra.Line(ps2)
						ra.Start(fs2)
						ra.Line(s2)
						return
					}
				}
			}
			// fallthrough to final JoinGap
		}
	}
	r.JoinGap(ra, p.P, p.TNorm, p.LNorm)
	ra.Line(s2)
	return
}

// Stop a stroked line. The line will close
// is isClosed is true. Otherwise end caps will
// be drawn at both ends.
func (r *Stroker) Stop(isClosed bool) {
	if r.inStroke == false {
		return
	}
	rf := &r.Filler
	if isClosed {
		if r.firstP.P != rf.a {
			r.Line(r.firstP.P)
		}
		a := rf.a
		r.firstP.TNorm = r.leadPoint.TNorm
		r.firstP.RT = r.leadPoint.RT
		r.firstP.TTan = r.leadPoint.TTan

		rf.Start(r.firstP.P.Sub(r.firstP.TNorm))
		rf.Line(a.Sub(r.ln))
		rf.Start(a.Add(r.ln))
		rf.Line(r.firstP.P.Add(r.firstP.TNorm))
		r.Joiner(r.firstP)
		r.firstP.blackWidowMark(rf)
	} else {
		a := rf.a
		rf.Start(r.leadPoint.P.Sub(r.leadPoint.TNorm))
		rf.Line(a.Sub(r.ln))
		rf.Start(a.Add(r.ln))
		rf.Line(r.leadPoint.P.Add(r.leadPoint.TNorm))
		r.CapL(rf, r.leadPoint.P, r.leadPoint.TNorm)
		r.CapT(rf, r.firstP.P, Invert(r.firstP.LNorm))
	}
	r.inStroke = false
}

// QuadBezier starts a stroked quadratic bezier.
func (r *Stroker) QuadBezier(b, c fixed.Point26_6) {
	r.quadBezierf(r, b, c)
}

// CubeBezier starts a stroked quadratic bezier.
func (r *Stroker) CubeBezier(b, c, d fixed.Point26_6) {
	r.cubeBezierf(r, b, c, d)
}

// quadBezierf calcs end curvature of beziers
func (r *Stroker) quadBezierf(s Rasterx, b, c fixed.Point26_6) {
	r.trailPoint = r.leadPoint
	r.CalcEndCurvature(r.a, b, c, c, b, r.a, fixed.Int52_12(2<<12), doCalcCurvature(s))
	r.QuadBezierF(s, b, c)
	r.a = c
}

// doCalcCurvature determines if calculation of the end curvature is required
// depending on the raster type and JoinMode
func doCalcCurvature(r Rasterx) bool {
	switch q := r.(type) {
	case *Filler:
		return false // never for filler
	case *Stroker:
		return (q.JoinMode == Arc || q.JoinMode == ArcClip)
	case *Dasher:
		return (q.JoinMode == Arc || q.JoinMode == ArcClip)
	default:
		return true // Better safe than sorry if another raster type is used
	}
}

func (r *Stroker) cubeBezierf(sgm Rasterx, b, c, d fixed.Point26_6) {
	if (r.a == b && c == d) || (r.a == b && b == c) || (c == b && d == c) {
		sgm.Line(d)
		return
	}
	r.trailPoint = r.leadPoint
	// Only calculate curvature if stroking or and using arc or arc-clip
	doCalcCurve := doCalcCurvature(sgm)
	const dm = fixed.Int52_12((3 << 12) / 2)
	switch {
	// b != c, and c != d see above
	case r.a == b:
		r.CalcEndCurvature(b, c, d, d, c, b, dm, doCalcCurve)
	// b != a,  and b != c, see above
	case c == d:
		r.CalcEndCurvature(r.a, b, c, c, b, r.a, dm, doCalcCurve)
	default:
		r.CalcEndCurvature(r.a, b, c, d, c, b, dm, doCalcCurve)
	}
	r.CubeBezierF(sgm, b, c, d)
	r.a = d
}

// Line adds a line segment to the rasterizer
func (r *Stroker) Line(b fixed.Point26_6) {
	r.LineSeg(r, b)
}

//LineSeg is called by both the Stroker and Dasher
func (r *Stroker) LineSeg(sgm Rasterx, b fixed.Point26_6) {
	r.trailPoint = r.leadPoint
	ba := b.Sub(r.a)
	if ba.X == 0 && ba.Y == 0 { // a == b, line is degenerate
		if r.trailPoint.TTan.X != 0 || r.trailPoint.TTan.Y != 0 {
			ba = r.trailPoint.TTan // Use last tangent for seg tangent
		} else { // Must be on top of last moveto; set ba to X axis unit vector
			ba = fixed.Point26_6{X: 1 << 6, Y: 0}
		}
	}
	bnorm := turnPort90(ToLength(ba, r.u))
	r.trailPoint.LTan = ba
	r.leadPoint.TTan = ba
	r.trailPoint.LNorm = bnorm
	r.leadPoint.TNorm = bnorm
	r.trailPoint.RL = 0.0
	r.leadPoint.RT = 0.0
	r.trailPoint.P = r.a
	r.leadPoint.P = b

	sgm.joinF()
	sgm.lineF(b)
	r.a = b
}

// lineF is for intra-curve lines. It is required for the Rasterizer interface
// so that if the line is being stroked or dash stroked, different actions can be
// taken.
func (r *Stroker) lineF(b fixed.Point26_6) {
	// b is either an intra-segment value, or
	// the end of the segment.
	var bnorm fixed.Point26_6
	a := r.a                // Hold a since r.a is going to change during stroke operation
	if b == r.leadPoint.P { // End of segment
		bnorm = r.leadPoint.TNorm // Use more accurate leadPoint tangent
	} else {
		bnorm = turnPort90(ToLength(b.Sub(a), r.u)) // Intra segment normal
	}
	ra := &r.Filler
	ra.Start(b.Sub(bnorm))
	ra.Line(a.Sub(r.ln))
	ra.Start(a.Add(r.ln))
	ra.Line(b.Add(bnorm))
	r.a = b
	r.ln = bnorm
}

// Start iniitates a stroked path
func (r *Stroker) Start(a fixed.Point26_6) {
	r.inStroke = false
	r.Filler.Start(a)
}

// CalcEndCurvature calculates the radius of curvature given the control points
// of a bezier curve.
// It is a low level function exposed for the purposes of callbacks
// and debugging.
func (r *Stroker) CalcEndCurvature(p0, p1, p2, q0, q1, q2 fixed.Point26_6,
	dm fixed.Int52_12, calcRadCuve bool) {
	r.trailPoint.P = p0
	r.leadPoint.P = q0
	r.trailPoint.LTan = p1.Sub(p0)
	r.leadPoint.TTan = q0.Sub(q1)
	r.trailPoint.LNorm = turnPort90(ToLength(r.trailPoint.LTan, r.u))
	r.leadPoint.TNorm = turnPort90(ToLength(r.leadPoint.TTan, r.u))
	if calcRadCuve {
		r.trailPoint.RL = RadCurvature(p0, p1, p2, dm)
		r.leadPoint.RT = -RadCurvature(q0, q1, q2, dm)
	} else {
		r.trailPoint.RL = 0
		r.leadPoint.RT = 0
	}
}

func (r *Stroker) joinF() {
	if r.inStroke == false {
		r.inStroke = true
		r.firstP = r.trailPoint
	} else {
		ra := &r.Filler
		tl := r.trailPoint.P.Sub(r.trailPoint.TNorm)
		th := r.trailPoint.P.Add(r.trailPoint.TNorm)
		if r.a != r.trailPoint.P || r.ln != r.trailPoint.TNorm {
			a := r.a
			ra.Start(tl)
			ra.Line(a.Sub(r.ln))
			ra.Start(a.Add(r.ln))
			ra.Line(th)
		}
		r.Joiner(r.trailPoint)
		r.trailPoint.blackWidowMark(ra)
	}
	r.ln = r.trailPoint.LNorm
	r.a = r.trailPoint.P
}

// blackWidowMark handles a gap in a stroke that can occur when a line end is too close
// to a segment to segment join point. Although it is only required in those cases,
// at this point, no code has been written to properly detect when it is needed,
// so for now it just draws by default.
func (jp *C2Point) blackWidowMark(ra Adder) {
	xprod := jp.TNorm.X*jp.LNorm.Y - jp.TNorm.Y*jp.LNorm.X
	if xprod > epsilonFixed*epsilonFixed {
		tl := jp.P.Sub(jp.TNorm)
		ll := jp.P.Sub(jp.LNorm)
		ra.Start(jp.P)
		ra.Line(tl)
		ra.Line(ll)
		ra.Line(jp.P)
	} else if xprod < -epsilonFixed*epsilonFixed {
		th := jp.P.Add(jp.TNorm)
		lh := jp.P.Add(jp.LNorm)
		ra.Start(jp.P)
		ra.Line(lh)
		ra.Line(th)
		ra.Line(jp.P)
	}
}
