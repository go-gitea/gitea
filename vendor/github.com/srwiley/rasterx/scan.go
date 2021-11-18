// Package rasterx implements a rasterizer in go.
// By default rasterx uses ScannerGV to render images
// which uses the rasterizer in the golang.org/x/image/vector package.
// The freetype rasterizer under the GNU license can also be used, by
// downloading the scanFT package.
//
// Copyright 2018 All rights reserved.
// Created: 5/12/2018 by S.R.Wiley
package rasterx

import (
	"image"
	"math"

	"image/color"
	"image/draw"

	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// At returns the color at the point x,y
func (c *ColorFuncImage) At(x, y int) color.Color {
	return c.colorFunc(x, y)
}

type (
	// ColorFuncImage implements and image
	// using the provided colorFunc
	ColorFuncImage struct {
		image.Uniform
		colorFunc ColorFunc
	}

	// ScannerGV uses the google vector rasterizer
	ScannerGV struct {
		r vector.Rasterizer
		//a, first fixed.Point26_6
		Dest                   draw.Image
		Targ                   image.Rectangle
		clipImage              *ClipImage
		Source                 image.Image
		Offset                 image.Point
		minX, minY, maxX, maxY fixed.Int26_6 // keep track of bounds
	}
)

// ClipImage is a clipable ColorFuncImage
type ClipImage struct {
	ColorFuncImage
	clip image.Rectangle
}

var noApha = color.RGBA{0, 0, 0, 0}

// GetPathExtent returns the extent of the path
func (s *ScannerGV) GetPathExtent() fixed.Rectangle26_6 {
	return fixed.Rectangle26_6{Min: fixed.Point26_6{X: s.minX, Y: s.minY}, Max: fixed.Point26_6{X: s.maxX, Y: s.maxY}}
}

// At returns the color of the ClipImage at the point x,y
func (c *ClipImage) At(x, y int) color.Color {
	p := image.Point{x, y}
	if p.In(c.clip) {
		return c.ColorFuncImage.At(x, y)
	}
	return noApha
}

// SetWinding set the winding rule for the scanner
func (s *ScannerGV) SetWinding(useNonZeroWinding bool) {
	// no-op as scanner gv does not support even-odd winding
}

// SetColor set the color type for the scanner
func (s *ScannerGV) SetColor(clr interface{}) {
	switch c := clr.(type) {
	case color.Color:
		s.clipImage.ColorFuncImage.Uniform.C = c
		if s.clipImage.clip == image.ZR {
			s.Source = &s.clipImage.ColorFuncImage.Uniform
		} else {
			s.clipImage.ColorFuncImage.colorFunc = func(x, y int) color.Color {
				return c
			}
			s.Source = s.clipImage
		}
	case ColorFunc:
		s.clipImage.ColorFuncImage.colorFunc = c
		if s.clipImage.clip == image.ZR {
			s.Source = &s.clipImage.ColorFuncImage
		} else {
			s.Source = s.clipImage
		}
	}
}

// SetClip sets an optional clipping rectangle to restrict rendering only to
// that region -- if size is 0 then ignored (set to image.ZR to clear)
func (s *ScannerGV) SetClip(rect image.Rectangle) {
	s.clipImage.clip = rect
	if s.Source == &s.clipImage.ColorFuncImage.Uniform {
		s.SetColor(s.clipImage.ColorFuncImage.Uniform.C)
	} else {
		s.SetColor(s.clipImage.ColorFuncImage.colorFunc)
	}
}

func (s *ScannerGV) set(a fixed.Point26_6) {
	if s.maxX < a.X {
		s.maxX = a.X
	}
	if s.maxY < a.Y {
		s.maxY = a.Y
	}
	if s.minX > a.X {
		s.minX = a.X
	}
	if s.minY > a.Y {
		s.minY = a.Y
	}
}

// Start starts a new path at the given point.
func (s *ScannerGV) Start(a fixed.Point26_6) {
	s.set(a)
	s.r.MoveTo(float32(a.X)/64, float32(a.Y)/64)
}

// Line adds a linear segment to the current curve.
func (s *ScannerGV) Line(b fixed.Point26_6) {
	s.set(b)
	s.r.LineTo(float32(b.X)/64, float32(b.Y)/64)
}

// Draw renders the accumulate scan to the desination
func (s *ScannerGV) Draw() {
	// This draws the entire bounds of the image, because
	// at this point the alpha mask does not shift with the
	// placement of the target rectangle in the vector rasterizer
	s.r.Draw(s.Dest, s.Dest.Bounds(), s.Source, s.Offset)

	// Remove the line above and uncomment the lines below if you
	// are using a version of the vector rasterizer that shifts the alpha
	// mask with the placement of the target

	//	s.Targ.Min.X = int(s.minX >> 6)
	//	s.Targ.Min.Y = int(s.minY >> 6)
	//	s.Targ.Max.X = int(s.maxX>>6) + 1
	//	s.Targ.Max.Y = int(s.maxY>>6) + 1
	//	s.Targ = s.Targ.Intersect(s.Dest.Bounds())  // This check should be done by the rasterizer?
	//	s.r.Draw(s.Dest, s.Targ, s.Source, s.Offset)
}

// Clear cancels any previous accumulated scans
func (s *ScannerGV) Clear() {
	p := s.r.Size()
	s.r.Reset(p.X, p.Y)
	const mxfi = fixed.Int26_6(math.MaxInt32)
	s.minX, s.minY, s.maxX, s.maxY = mxfi, mxfi, -mxfi, -mxfi
}

// SetBounds sets the maximum width and height of the rasterized image and
// calls Clear. The width and height are in pixels, not fixed.Int26_6 units.
func (s *ScannerGV) SetBounds(width, height int) {
	s.r.Reset(width, height)
}

// NewScannerGV creates a new Scanner with the given bounds.
func NewScannerGV(width, height int, dest draw.Image,
	targ image.Rectangle) *ScannerGV {
	s := new(ScannerGV)
	s.SetBounds(width, height)
	s.Dest = dest
	s.Targ = targ
	s.clipImage = &ClipImage{}
	s.clipImage.ColorFuncImage.Uniform.C = &color.RGBA{255, 0, 0, 255}
	s.Source = &s.clipImage.ColorFuncImage.Uniform
	s.Offset = image.Point{0, 0}
	return s
}
