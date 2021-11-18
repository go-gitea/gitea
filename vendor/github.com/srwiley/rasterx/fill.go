// Copyright 2018 by the rasterx Authors. All rights reserved.
//_
// Created 2017 by S.R.Wiley

package rasterx

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/math/fixed"
)

type (
	// ColorFunc maps a color to x y coordinates
	ColorFunc func(x, y int) color.Color
	// Scanner interface for path generating types
	Scanner interface {
		Start(a fixed.Point26_6)
		Line(b fixed.Point26_6)
		Draw()
		GetPathExtent() fixed.Rectangle26_6
		SetBounds(w, h int)
		SetColor(color interface{})
		SetWinding(useNonZeroWinding bool)
		Clear()

		// SetClip sets an optional clipping rectangle to restrict rendering
		// only to that region -- if size is 0 then ignored (set to image.ZR
		// to clear)
		SetClip(rect image.Rectangle)
	}
	// Adder interface for types that can accumlate path commands
	Adder interface {
		// Start starts a new curve at the given point.
		Start(a fixed.Point26_6)
		// Line adds a line segment to the path
		Line(b fixed.Point26_6)
		// QuadBezier adds a quadratic bezier curve to the path
		QuadBezier(b, c fixed.Point26_6)
		// CubeBezier adds a cubic bezier curve to the path
		CubeBezier(b, c, d fixed.Point26_6)
		// Closes the path to the start point if closeLoop is true
		Stop(closeLoop bool)
	}
	// Rasterx extends the adder interface to include lineF and joinF functions
	Rasterx interface {
		Adder
		lineF(b fixed.Point26_6)
		joinF()
	}

	// Filler satisfies Rasterx
	Filler struct {
		Scanner
		a, first fixed.Point26_6
	}
)

// Start starts a new path at the given point.
func (r *Filler) Start(a fixed.Point26_6) {
	r.a = a
	r.first = a
	r.Scanner.Start(a)
}

// Stop sends a path at the given point.
func (r *Filler) Stop(isClosed bool) {
	if r.first != r.a {
		r.Line(r.first)
	}
}

// QuadBezier adds a quadratic segment to the current curve.
func (r *Filler) QuadBezier(b, c fixed.Point26_6) {
	r.QuadBezierF(r, b, c)
}

// QuadTo flattens the quadratic Bezier curve into lines through the LineTo func
// This functions is adapted from the version found in
// golang.org/x/image/vector
func QuadTo(ax, ay, bx, by, cx, cy float32, LineTo func(dx, dy float32)) {
	devsq := devSquared(ax, ay, bx, by, cx, cy)
	if devsq >= 0.333 {
		const tol = 3
		n := 1 + int(math.Sqrt(math.Sqrt(tol*float64(devsq))))
		t, nInv := float32(0), 1/float32(n)
		for i := 0; i < n-1; i++ {
			t += nInv

			mt := 1 - t
			t1 := mt * mt
			t2 := mt * t * 2
			t3 := t * t
			LineTo(
				ax*t1+bx*t2+cx*t3,
				ay*t1+by*t2+cy*t3)
		}
	}
	LineTo(cx, cy)
}

// CubeTo flattens the cubic Bezier curve into lines through the LineTo func
// This functions is adapted from the version found in
// golang.org/x/image/vector
func CubeTo(ax, ay, bx, by, cx, cy, dx, dy float32, LineTo func(ex, ey float32)) {
	devsq := devSquared(ax, ay, bx, by, dx, dy)
	if devsqAlt := devSquared(ax, ay, cx, cy, dx, dy); devsq < devsqAlt {
		devsq = devsqAlt
	}
	if devsq >= 0.333 {
		const tol = 3
		n := 1 + int(math.Sqrt(math.Sqrt(tol*float64(devsq))))
		t, nInv := float32(0), 1/float32(n)
		for i := 0; i < n-1; i++ {
			t += nInv

			tsq := t * t
			mt := 1 - t
			mtsq := mt * mt
			t1 := mtsq * mt
			t2 := mtsq * t * 3
			t3 := mt * tsq * 3
			t4 := tsq * t
			LineTo(
				ax*t1+bx*t2+cx*t3+dx*t4,
				ay*t1+by*t2+cy*t3+dy*t4)
		}
	}
	LineTo(dx, dy)
}

// devSquared returns a measure of how curvy the sequence (ax, ay) to (bx, by)
// to (cx, cy) is. It determines how many line segments will approximate a
// Bézier curve segment. This functions is copied from the version found in
// golang.org/x/image/vector as are the below comments.
//
// http://lists.nongnu.org/archive/html/freetype-devel/2016-08/msg00080.html
// gives the rationale for this evenly spaced heuristic instead of a recursive
// de Casteljau approach:
//
// The reason for the subdivision by n is that I expect the "flatness"
// computation to be semi-expensive (it's done once rather than on each
// potential subdivision) and also because you'll often get fewer subdivisions.
// Taking a circular arc as a simplifying assumption (ie a spherical cow),
// where I get n, a recursive approach would get 2^⌈lg n⌉, which, if I haven't
// made any horrible mistakes, is expected to be 33% more in the limit.
func devSquared(ax, ay, bx, by, cx, cy float32) float32 {
	devx := ax - 2*bx + cx
	devy := ay - 2*by + cy
	return devx*devx + devy*devy
}

// QuadBezierF adds a quadratic segment to the sgm Rasterizer.
func (r *Filler) QuadBezierF(sgm Rasterx, b, c fixed.Point26_6) {
	// check for degenerate bezier
	if r.a == b || b == c {
		sgm.Line(c)
		return
	}
	sgm.joinF()
	QuadTo(float32(r.a.X), float32(r.a.Y), // Pts are x64, but does not matter.
		float32(b.X), float32(b.Y),
		float32(c.X), float32(c.Y),
		func(dx, dy float32) {
			sgm.lineF(fixed.Point26_6{X: fixed.Int26_6(dx), Y: fixed.Int26_6(dy)})
		})

}

// CubeBezier adds a cubic bezier to the curve
func (r *Filler) CubeBezier(b, c, d fixed.Point26_6) {
	r.CubeBezierF(r, b, c, d)
}

// joinF is a no-op for a filling rasterizer. This is used in stroking and dashed
// stroking
func (r *Filler) joinF() {

}

// Line for a filling rasterizer is just the line call in scan
func (r *Filler) Line(b fixed.Point26_6) {
	r.lineF(b)
}

// lineF for a filling rasterizer is just the line call in scan
func (r *Filler) lineF(b fixed.Point26_6) {
	r.Scanner.Line(b)
	r.a = b
}

// CubeBezierF adds a cubic bezier to the curve. sending the line calls the the
// sgm Rasterizer
func (r *Filler) CubeBezierF(sgm Rasterx, b, c, d fixed.Point26_6) {
	if (r.a == b && c == d) || (r.a == b && b == c) || (c == b && d == c) {
		sgm.Line(d)
		return
	}
	sgm.joinF()
	CubeTo(float32(r.a.X), float32(r.a.Y),
		float32(b.X), float32(b.Y),
		float32(c.X), float32(c.Y),
		float32(d.X), float32(d.Y),
		func(ex, ey float32) {
			sgm.lineF(fixed.Point26_6{X: fixed.Int26_6(ex), Y: fixed.Int26_6(ey)})
		})
}

// Clear resets the filler
func (r *Filler) Clear() {
	r.a = fixed.Point26_6{}
	r.first = r.a
	r.Scanner.Clear()
}

// SetBounds sets the maximum width and height of the rasterized image and
// calls Clear. The width and height are in pixels, not fixed.Int26_6 units.
func (r *Filler) SetBounds(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	r.Scanner.SetBounds(width, height)
	r.Clear()
}

// NewFiller returns a Filler ptr with default values.
// A Filler in addition to rasterizing lines like a Scann,
// can also rasterize quadratic and cubic bezier curves.
// If Scanner is nil default scanner ScannerGV is used
func NewFiller(width, height int, scanner Scanner) *Filler {
	r := new(Filler)
	r.Scanner = scanner
	r.SetBounds(width, height)
	r.SetWinding(true)
	return r
}
