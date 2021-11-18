// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley
//
// svgd.go implements translation of an SVG2.0 path into a rasterx Path.

package oksvg

import (
	"errors"
	"log"
	"math"
	"unicode"

	"github.com/srwiley/rasterx"

	"golang.org/x/image/math/fixed"
)

type (
	//ErrorMode is the for setting how the parser reacts to unparsed elements
	ErrorMode uint8
	// PathCursor is used to parse SVG format path strings into a rasterx Path
	PathCursor struct {
		rasterx.Path
		placeX, placeY         float64
		curX, curY             float64
		cntlPtX, cntlPtY       float64
		pathStartX, pathStartY float64
		points                 []float64
		lastKey                uint8
		ErrorMode              ErrorMode
		inPath                 bool
	}
)

var (
	errParamMismatch  = errors.New("param mismatch")
	errCommandUnknown = errors.New("unknown command")
	errZeroLengthID   = errors.New("zero length id")
)

const (
	//IgnoreErrorMode skips unparsed SVG elements
	IgnoreErrorMode ErrorMode = iota
	//WarnErrorMode outputs a warning when an unparsed SVG element is found
	WarnErrorMode
	//StrictErrorMode causes a error when an unparsed SVG element is found
	StrictErrorMode
)

func reflect(px, py, rx, ry float64) (x, y float64) {
	return px*2 - rx, py*2 - ry
}

func (c *PathCursor) valsToAbs(last float64) {
	for i := 0; i < len(c.points); i++ {
		last += c.points[i]
		c.points[i] = last
	}
}

func (c *PathCursor) pointsToAbs(sz int) {
	lastX := c.placeX
	lastY := c.placeY
	for j := 0; j < len(c.points); j += sz {
		for i := 0; i < sz; i += 2 {
			c.points[i+j] += lastX
			c.points[i+1+j] += lastY
		}
		lastX = c.points[(j+sz)-2]
		lastY = c.points[(j+sz)-1]
	}
}

func (c *PathCursor) hasSetsOrMore(sz int, rel bool) bool {
	if !(len(c.points) >= sz && len(c.points)%sz == 0) {
		return false
	}
	if rel {
		c.pointsToAbs(sz)
	}
	return true
}

// ReadFloat reads a floating point value and adds it to the cursor's points slice.
func (c *PathCursor) ReadFloat(numStr string) error {
	last := 0
	isFirst := true
	for i, n := range numStr {
		if n == '.' {
			if isFirst {
				isFirst = false
				continue
			}
			f, err := parseFloat(numStr[last:i], 64)
			if err != nil {
				return err
			}
			c.points = append(c.points, f)
			last = i
		}
	}
	f, err := parseFloat(numStr[last:], 64)
	if err != nil {
		return err
	}
	c.points = append(c.points, f)
	return nil
}

// GetPoints reads a set of floating point values from the SVG format number string,
// and add them to the cursor's points slice.
func (c *PathCursor) GetPoints(dataPoints string) error {
	lastIndex := -1
	c.points = c.points[0:0]
	lr := ' '
	for i, r := range dataPoints {
		if !unicode.IsNumber(r) && r != '.' && !(r == '-' && lr == 'e') && r != 'e' {
			if lastIndex != -1 {
				if err := c.ReadFloat(dataPoints[lastIndex:i]); err != nil {
					return err
				}
			}
			if r == '-' {
				lastIndex = i
			} else {
				lastIndex = -1
			}
		} else if lastIndex == -1 {
			lastIndex = i
		}
		lr = r
	}
	if lastIndex != -1 && lastIndex != len(dataPoints) {
		if err := c.ReadFloat(dataPoints[lastIndex:]); err != nil {
			return err
		}
	}
	return nil
}

func (c *PathCursor) reflectControlQuad() {
	switch c.lastKey {
	case 'q', 'Q', 'T', 't':
		c.cntlPtX, c.cntlPtY = reflect(c.placeX, c.placeY, c.cntlPtX, c.cntlPtY)
	default:
		c.cntlPtX, c.cntlPtY = c.placeX, c.placeY
	}
}

func (c *PathCursor) reflectControlCube() {
	switch c.lastKey {
	case 'c', 'C', 's', 'S':
		c.cntlPtX, c.cntlPtY = reflect(c.placeX, c.placeY, c.cntlPtX, c.cntlPtY)
	default:
		c.cntlPtX, c.cntlPtY = c.placeX, c.placeY
	}
}

// addSeg decodes an SVG seqment string into equivalent raster path commands saved
// in the cursor's Path
func (c *PathCursor) addSeg(segString string) error {
	// Parse the string describing the numeric points in SVG format
	if err := c.GetPoints(segString[1:]); err != nil {
		return err
	}
	l := len(c.points)
	k := segString[0]
	rel := false
	switch k {
	case 'z':
		fallthrough
	case 'Z':
		if len(c.points) != 0 {
			return errParamMismatch
		}
		if c.inPath {
			c.Path.Stop(true)
			c.placeX = c.pathStartX
			c.placeY = c.pathStartY
			c.inPath = false
		}
	case 'm':
		rel = true
		fallthrough
	case 'M':
		if !c.hasSetsOrMore(2, rel) {
			return errParamMismatch
		}
		c.pathStartX, c.pathStartY = c.points[0], c.points[1]
		c.inPath = true
		c.Path.Start(fixed.Point26_6{X: fixed.Int26_6((c.pathStartX + c.curX) * 64), Y: fixed.Int26_6((c.pathStartY + c.curY) * 64)})
		for i := 2; i < l-1; i += 2 {
			c.Path.Line(fixed.Point26_6{
				X: fixed.Int26_6((c.points[i] + c.curX) * 64),
				Y: fixed.Int26_6((c.points[i+1] + c.curY) * 64)})
		}
		c.placeX = c.points[l-2]
		c.placeY = c.points[l-1]
	case 'l':
		rel = true
		fallthrough
	case 'L':
		if !c.hasSetsOrMore(2, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-1; i += 2 {
			c.Path.Line(fixed.Point26_6{
				X: fixed.Int26_6((c.points[i] + c.curX) * 64),
				Y: fixed.Int26_6((c.points[i+1] + c.curY) * 64)})
		}
		c.placeX = c.points[l-2]
		c.placeY = c.points[l-1]
	case 'v':
		c.valsToAbs(c.placeY)
		fallthrough
	case 'V':
		if !c.hasSetsOrMore(1, false) {
			return errParamMismatch
		}
		for _, p := range c.points {
			c.Path.Line(fixed.Point26_6{
				X: fixed.Int26_6((c.placeX + c.curX) * 64),
				Y: fixed.Int26_6((p + c.curY) * 64)})
		}
		c.placeY = c.points[l-1]
	case 'h':
		c.valsToAbs(c.placeX)
		fallthrough
	case 'H':
		if !c.hasSetsOrMore(1, false) {
			return errParamMismatch
		}
		for _, p := range c.points {
			c.Path.Line(fixed.Point26_6{
				X: fixed.Int26_6((p + c.curX) * 64),
				Y: fixed.Int26_6((c.placeY + c.curY) * 64)})
		}
		c.placeX = c.points[l-1]
	case 'q':
		rel = true
		fallthrough
	case 'Q':
		if !c.hasSetsOrMore(4, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-3; i += 4 {
			c.Path.QuadBezier(
				fixed.Point26_6{
					X: fixed.Int26_6((c.points[i] + c.curX) * 64),
					Y: fixed.Int26_6((c.points[i+1] + c.curY) * 64)},
				fixed.Point26_6{
					X: fixed.Int26_6((c.points[i+2] + c.curX) * 64),
					Y: fixed.Int26_6((c.points[i+3] + c.curY) * 64)})
		}
		c.cntlPtX, c.cntlPtY = c.points[l-4], c.points[l-3]
		c.placeX = c.points[l-2]
		c.placeY = c.points[l-1]
	case 't':
		rel = true
		fallthrough
	case 'T':
		if !c.hasSetsOrMore(2, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-1; i += 2 {
			c.reflectControlQuad()
			c.Path.QuadBezier(
				fixed.Point26_6{
					X: fixed.Int26_6((c.cntlPtX + c.curX) * 64),
					Y: fixed.Int26_6((c.cntlPtY + c.curY) * 64)},
				fixed.Point26_6{
					X: fixed.Int26_6((c.points[i] + c.curX) * 64),
					Y: fixed.Int26_6((c.points[i+1] + c.curY) * 64)})
			c.lastKey = k
			c.placeX = c.points[i]
			c.placeY = c.points[i+1]
		}
	case 'c':
		rel = true
		fallthrough
	case 'C':
		if !c.hasSetsOrMore(6, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-5; i += 6 {
			c.Path.CubeBezier(
				fixed.Point26_6{
					X: fixed.Int26_6((c.points[i] + c.curX) * 64),
					Y: fixed.Int26_6((c.points[i+1] + c.curY) * 64)},
				fixed.Point26_6{
					X: fixed.Int26_6((c.points[i+2] + c.curX) * 64),
					Y: fixed.Int26_6((c.points[i+3] + c.curY) * 64)},
				fixed.Point26_6{
					X: fixed.Int26_6((c.points[i+4] + c.curX) * 64),
					Y: fixed.Int26_6((c.points[i+5] + c.curY) * 64)})
		}
		c.cntlPtX, c.cntlPtY = c.points[l-4], c.points[l-3]
		c.placeX = c.points[l-2]
		c.placeY = c.points[l-1]
	case 's':
		rel = true
		fallthrough
	case 'S':
		if !c.hasSetsOrMore(4, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-3; i += 4 {
			c.reflectControlCube()
			c.Path.CubeBezier(fixed.Point26_6{
				X: fixed.Int26_6((c.cntlPtX + c.curX) * 64), Y: fixed.Int26_6((c.cntlPtY + c.curY) * 64)},
				fixed.Point26_6{
					X: fixed.Int26_6((c.points[i] + c.curX) * 64), Y: fixed.Int26_6((c.points[i+1] + c.curY) * 64)},
				fixed.Point26_6{
					X: fixed.Int26_6((c.points[i+2] + c.curX) * 64), Y: fixed.Int26_6((c.points[i+3] + c.curY) * 64)})
			c.lastKey = k
			c.cntlPtX, c.cntlPtY = c.points[i], c.points[i+1]
			c.placeX = c.points[i+2]
			c.placeY = c.points[i+3]
		}
	case 'a', 'A':
		if !c.hasSetsOrMore(7, false) {
			return errParamMismatch
		}
		for i := 0; i < l-6; i += 7 {
			if k == 'a' {
				c.points[i+5] += c.placeX
				c.points[i+6] += c.placeY
			}
			c.AddArcFromA(c.points[i:])
		}
	default:
		if c.ErrorMode == StrictErrorMode {
			return errCommandUnknown
		}
		if c.ErrorMode == WarnErrorMode {
			log.Println("Ignoring svg command " + string(k))
		}
	}
	// So we know how to extend some segment types
	c.lastKey = k
	return nil
}

//EllipseAt adds a path of an elipse centered at cx, cy of radius rx and ry
// to the PathCursor
func (c *PathCursor) EllipseAt(cx, cy, rx, ry float64) {
	c.placeX, c.placeY = cx+rx, cy
	c.points = c.points[0:0]
	c.points = append(c.points, rx, ry, 0.0, 1.0, 0.0, c.placeX, c.placeY)
	c.Path.Start(fixed.Point26_6{
		X: fixed.Int26_6(c.placeX * 64),
		Y: fixed.Int26_6(c.placeY * 64)})
	c.placeX, c.placeY = rasterx.AddArc(c.points, cx, cy, c.placeX, c.placeY, &c.Path)
	c.Path.Stop(true)
}

//AddArcFromA adds a path of an arc element to the cursor path to the PathCursor
func (c *PathCursor) AddArcFromA(points []float64) {
	cx, cy := rasterx.FindEllipseCenter(&points[0], &points[1], points[2]*math.Pi/180, c.placeX,
		c.placeY, points[5], points[6], points[4] == 0, points[3] == 0)
	c.placeX, c.placeY = rasterx.AddArc(c.points, cx+c.curX, cy+c.curY, c.placeX+c.curX, c.placeY+c.curY, &c.Path)
}

func (c *PathCursor) init() {
	c.placeX = 0.0
	c.placeY = 0.0
	c.points = c.points[0:0]
	c.lastKey = ' '
	c.Path.Clear()
	c.inPath = false
}

// CompilePath translates the svgPath description string into a rasterx path.
// All valid SVG path elements are interpreted to rasterx equivalents.
// The resulting path element is stored in the PathCursor.
func (c *PathCursor) CompilePath(svgPath string) error {
	c.init()
	lastIndex := -1
	for i, v := range svgPath {
		if unicode.IsLetter(v) && v != 'e' {
			if lastIndex != -1 {
				if err := c.addSeg(svgPath[lastIndex:i]); err != nil {
					return err
				}
			}
			lastIndex = i
		}
	}
	if lastIndex != -1 {
		if err := c.addSeg(svgPath[lastIndex:]); err != nil {
			return err
		}
	}
	return nil
}
