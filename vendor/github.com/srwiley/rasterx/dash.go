// Copyright 2017 by the rasterx Authors. All rights reserved.
//_
// created: 2017 by S.R.Wiley

package rasterx

import (
	"golang.org/x/image/math/fixed"
)

// Dasher struct extends the Stroker and can draw
// dashed lines with end capping
type Dasher struct {
	Stroker
	Dashes                    []fixed.Int26_6
	dashPlace                 int
	firstDashIsGap, dashIsGap bool
	deltaDash, DashOffset     fixed.Int26_6
	sgm                       Rasterx
	// sgm allows us to switch between dashing
	// and non-dashing rasterizers in the SetStroke function.
}

// joinF overides stroker joinF during dashed stroking, because we need to slightly modify
// the the call as below to handle the case of the join being in a dash gap.
func (r *Dasher) joinF() {
	if len(r.Dashes) == 0 || !r.inStroke || !r.dashIsGap {
		r.Stroker.joinF()
	}
}

// Start starts a dashed line
func (r *Dasher) Start(a fixed.Point26_6) {
	// Advance dashPlace to the dashOffset start point and set deltaDash
	if len(r.Dashes) > 0 {
		r.deltaDash = r.DashOffset
		r.dashIsGap = false
		r.dashPlace = 0
		for r.deltaDash > r.Dashes[r.dashPlace] {
			r.deltaDash -= r.Dashes[r.dashPlace]
			r.dashIsGap = !r.dashIsGap
			r.dashPlace++
			if r.dashPlace == len(r.Dashes) {
				r.dashPlace = 0
			}
		}
		r.firstDashIsGap = r.dashIsGap
	}
	r.Stroker.Start(a)
}

// lineF overides stroker lineF to modify the the call as below
// while performing the join in a dashed stroke.
func (r *Dasher) lineF(b fixed.Point26_6) {
	var bnorm fixed.Point26_6
	a := r.a // Copy local a since r.a is going to change during stroke operation
	ba := b.Sub(a)
	segLen := Length(ba)
	var nlt fixed.Int26_6
	if b == r.leadPoint.P { // End of segment
		bnorm = r.leadPoint.TNorm // Use more accurate leadPoint tangent
	} else {
		bnorm = turnPort90(ToLength(b.Sub(a), r.u)) // Intra segment normal
	}
	for segLen+r.deltaDash > r.Dashes[r.dashPlace] {
		nl := r.Dashes[r.dashPlace] - r.deltaDash
		nlt += nl
		r.dashLineStrokeBit(a.Add(ToLength(ba, nlt)), bnorm, false)
		r.dashIsGap = !r.dashIsGap
		segLen -= nl
		r.deltaDash = 0
		r.dashPlace++
		if r.dashPlace == len(r.Dashes) {
			r.dashPlace = 0
		}
	}
	r.deltaDash += segLen
	r.dashLineStrokeBit(b, bnorm, true)
}

// SetStroke set the parameters for stroking a line. width is the width of the line, miterlimit is the miter cutoff
// value for miter, arc, miterclip and arcClip joinModes. CapL and CapT are the capping functions for leading and trailing
// line ends. If one is nil, the other function is used at both ends. gp is the gap function that determines how a
// gap on the convex side of two lines joining is filled. jm is the JoinMode for curve segments. Dashes is the values for
// the dash pattern. Pass in nil or an empty slice for no dashes. dashoffset is the starting offset into the dash array.
func (r *Dasher) SetStroke(width, miterLimit fixed.Int26_6, capL, capT CapFunc, gp GapFunc, jm JoinMode, dashes []float64, dashOffset float64) {
	r.Stroker.SetStroke(width, miterLimit, capL, capT, gp, jm)

	r.Dashes = r.Dashes[:0] // clear the dash array
	if len(dashes) == 0 {
		r.sgm = &r.Stroker // This is just plain stroking
		return
	}
	// Dashed Stroke
	// Convert the float dash array and offset to fixed point and attach to the Filler
	oneIsPos := false // Check to see if at least one dash is > 0
	for _, v := range dashes {
		fv := fixed.Int26_6(v * 64)
		if fv <= 0 { // Negatives are considered 0s.
			fv = 0
		} else {
			oneIsPos = true
		}
		r.Dashes = append(r.Dashes, fv)
	}
	if oneIsPos == false {
		r.Dashes = r.Dashes[:0]
		r.sgm = &r.Stroker // This is just plain stroking
		return
	}
	r.DashOffset = fixed.Int26_6(dashOffset * 64)
	r.sgm = r // Use the full dasher
}

//Stop terminates a dashed line
func (r *Dasher) Stop(isClosed bool) {
	if len(r.Dashes) == 0 {
		r.Stroker.Stop(isClosed)
		return
	}
	if r.inStroke == false {
		return
	}
	if isClosed && r.a != r.firstP.P {
		r.LineSeg(r.sgm, r.firstP.P)
	}
	ra := &r.Filler
	if isClosed && !r.firstDashIsGap && !r.dashIsGap { // closed connect w/o caps
		a := r.a
		r.firstP.TNorm = r.leadPoint.TNorm
		r.firstP.RT = r.leadPoint.RT
		r.firstP.TTan = r.leadPoint.TTan
		ra.Start(r.firstP.P.Sub(r.firstP.TNorm))
		ra.Line(a.Sub(r.ln))
		ra.Start(a.Add(r.ln))
		ra.Line(r.firstP.P.Add(r.firstP.TNorm))
		r.Joiner(r.firstP)
		r.firstP.blackWidowMark(ra)
	} else { // Cap open ends
		if !r.dashIsGap {
			r.CapL(ra, r.leadPoint.P, r.leadPoint.TNorm)
		}
		if !r.firstDashIsGap {
			r.CapT(ra, r.firstP.P, Invert(r.firstP.LNorm))
		}
	}
	r.inStroke = false
}

// dashLineStrokeBit is a helper function that reduces code redundancey in the
// lineF function.
func (r *Dasher) dashLineStrokeBit(b, bnorm fixed.Point26_6, dontClose bool) {
	if !r.dashIsGap { // Moving from dash to gap
		a := r.a
		ra := &r.Filler
		ra.Start(b.Sub(bnorm))
		ra.Line(a.Sub(r.ln))
		ra.Start(a.Add(r.ln))
		ra.Line(b.Add(bnorm))
		if dontClose == false {
			r.CapL(ra, b, bnorm)
		}
	} else { // Moving from gap to dash
		if dontClose == false {
			ra := &r.Filler
			r.CapT(ra, b, Invert(bnorm))
		}
	}
	r.a = b
	r.ln = bnorm
}

// Line for Dasher is here to pass the dasher sgm to LineP
func (r *Dasher) Line(b fixed.Point26_6) {
	r.LineSeg(r.sgm, b)
}

// QuadBezier for dashing
func (r *Dasher) QuadBezier(b, c fixed.Point26_6) {
	r.quadBezierf(r.sgm, b, c)
}

// CubeBezier starts a stroked cubic bezier.
// It is a low level function exposed for the purposes of callbacks
// and debugging.
func (r *Dasher) CubeBezier(b, c, d fixed.Point26_6) {
	r.cubeBezierf(r.sgm, b, c, d)
}

// NewDasher returns a Dasher ptr with default values.
// A Dasher has all of the capabilities of a Stroker, Filler, and Scanner, plus the ability
// to stroke curves with solid lines. Use SetStroke to configure with non-default
// values.
func NewDasher(width, height int, scanner Scanner) *Dasher {
	r := new(Dasher)
	r.Scanner = scanner
	r.SetBounds(width, height)
	r.SetWinding(true)
	r.SetStroke(1*64, 4*64, ButtCap, nil, FlatGap, MiterClip, nil, 0)
	r.sgm = &r.Stroker
	return r
}
