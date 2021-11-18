// Copyright 2017 The oksvg Authors. All rights reserved.
//
// created: 2/12/2017 by S.R.Wiley
// The oksvg package provides a partial implementation of the SVG 2.0 standard.
// It can perform all SVG2.0 path commands, including arc and miterclip. It also
// has some additional capabilities like arc-clip. Svgdraw does
// not implement all SVG features such as animation or markers, but it can draw
// the many of open source SVG icons correctly. See Readme for
// a list of features.

package oksvg

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/html/charset"

	"encoding/xml"
	"errors"
	"image/color"
	"log"
	"math"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/colornames"
	"golang.org/x/image/math/fixed"
)

type (
	// PathStyle holds the state of the SVG style
	PathStyle struct {
		FillOpacity, LineOpacity          float64
		LineWidth, DashOffset, MiterLimit float64
		Dash                              []float64
		UseNonZeroWinding                 bool
		fillerColor, linerColor           interface{} // either color.Color or rasterx.Gradient
		LineGap                           rasterx.GapFunc
		LeadLineCap                       rasterx.CapFunc // This is used if different than LineCap
		LineCap                           rasterx.CapFunc
		LineJoin                          rasterx.JoinMode
		mAdder                            rasterx.MatrixAdder // current transform
	}

	// SvgPath binds a style to a path
	SvgPath struct {
		PathStyle
		Path rasterx.Path
	}

	// SvgIcon holds data from parsed SVGs
	SvgIcon struct {
		ViewBox      struct{ X, Y, W, H float64 }
		Titles       []string // Title elements collect here
		Descriptions []string // Description elements collect here
		Grads        map[string]*rasterx.Gradient
		Defs         map[string][]definition
		SVGPaths     []SvgPath
		Transform    rasterx.Matrix2D
		classes      map[string]styleAttribute
	}

	// IconCursor is used while parsing SVG files
	IconCursor struct {
		PathCursor
		icon                                                 *SvgIcon
		StyleStack                                           []PathStyle
		grad                                                 *rasterx.Gradient
		inTitleText, inDescText, inGrad, inDefs, inDefsStyle bool
		currentDef                                           []definition
	}

	// definition is used to store what's given in a def tag
	definition struct {
		ID, Tag string
		Attrs   []xml.Attr
	}

	// styleAttribute describes draw options, such as {"fill":"black"; "stroke":"white"}
	styleAttribute = map[string]string
)

// DefaultStyle sets the default PathStyle to fill black, winding rule,
// full opacity, no stroke, ButtCap line end and Bevel line connect.
var DefaultStyle = PathStyle{1.0, 1.0, 2.0, 0.0, 4.0, nil, true,
	color.NRGBA{0x00, 0x00, 0x00, 0xff}, nil,
	nil, nil, rasterx.ButtCap, rasterx.Bevel, rasterx.MatrixAdder{M: rasterx.Identity}}

// Draw the compiled SVG icon into the GraphicContext.
// All elements should be contained by the Bounds rectangle of the SvgIcon.
func (s *SvgIcon) Draw(r *rasterx.Dasher, opacity float64) {
	for _, svgp := range s.SVGPaths {
		svgp.DrawTransformed(r, opacity, s.Transform)
	}
}

// SetTarget sets the Transform matrix to draw within the bounds of the rectangle arguments
func (s *SvgIcon) SetTarget(x, y, w, h float64) {
	scaleW := w / s.ViewBox.W
	scaleH := h / s.ViewBox.H
	s.Transform = rasterx.Identity.Translate(x-s.ViewBox.X, y-s.ViewBox.Y).Scale(scaleW, scaleH)
}

// Draw the compiled SvgPath into the Dasher.
func (svgp *SvgPath) Draw(r *rasterx.Dasher, opacity float64) {
	svgp.DrawTransformed(r, opacity, rasterx.Identity)
}

// DrawTransformed draws the compiled SvgPath into the Dasher while applying transform t.
func (svgp *SvgPath) DrawTransformed(r *rasterx.Dasher, opacity float64, t rasterx.Matrix2D) {
	m := svgp.mAdder.M
	svgp.mAdder.M = t.Mult(m)
	defer func() { svgp.mAdder.M = m }() // Restore untransformed matrix
	if svgp.fillerColor != nil {
		r.Clear()
		rf := &r.Filler
		rf.SetWinding(svgp.UseNonZeroWinding)
		svgp.mAdder.Adder = rf // This allows transformations to be applied
		svgp.Path.AddTo(&svgp.mAdder)

		switch fillerColor := svgp.fillerColor.(type) {
		case color.Color:
			rf.SetColor(rasterx.ApplyOpacity(fillerColor, svgp.FillOpacity*opacity))
		case rasterx.Gradient:
			if fillerColor.Units == rasterx.ObjectBoundingBox {
				fRect := rf.Scanner.GetPathExtent()
				mnx, mny := float64(fRect.Min.X)/64, float64(fRect.Min.Y)/64
				mxx, mxy := float64(fRect.Max.X)/64, float64(fRect.Max.Y)/64
				fillerColor.Bounds.X, fillerColor.Bounds.Y = mnx, mny
				fillerColor.Bounds.W, fillerColor.Bounds.H = mxx-mnx, mxy-mny
			}
			rf.SetColor(fillerColor.GetColorFunction(svgp.FillOpacity * opacity))
		}
		rf.Draw()
		// default is true
		rf.SetWinding(true)
	}
	if svgp.linerColor != nil {
		r.Clear()
		svgp.mAdder.Adder = r
		lineGap := svgp.LineGap
		if lineGap == nil {
			lineGap = DefaultStyle.LineGap
		}
		lineCap := svgp.LineCap
		if lineCap == nil {
			lineCap = DefaultStyle.LineCap
		}
		leadLineCap := lineCap
		if svgp.LeadLineCap != nil {
			leadLineCap = svgp.LeadLineCap
		}
		r.SetStroke(fixed.Int26_6(svgp.LineWidth*64),
			fixed.Int26_6(svgp.MiterLimit*64), leadLineCap, lineCap,
			lineGap, svgp.LineJoin, svgp.Dash, svgp.DashOffset)
		svgp.Path.AddTo(&svgp.mAdder)
		switch linerColor := svgp.linerColor.(type) {
		case color.Color:
			r.SetColor(rasterx.ApplyOpacity(linerColor, svgp.LineOpacity*opacity))
		case rasterx.Gradient:
			if linerColor.Units == rasterx.ObjectBoundingBox {
				fRect := r.Scanner.GetPathExtent()
				mnx, mny := float64(fRect.Min.X)/64, float64(fRect.Min.Y)/64
				mxx, mxy := float64(fRect.Max.X)/64, float64(fRect.Max.Y)/64
				linerColor.Bounds.X, linerColor.Bounds.Y = mnx, mny
				linerColor.Bounds.W, linerColor.Bounds.H = mxx-mnx, mxy-mny
			}
			r.SetColor(linerColor.GetColorFunction(svgp.LineOpacity * opacity))
		}
		r.Draw()
	}
}

// GetFillColor returns the fill color of the SvgPath if one is defined and otherwise returns colornames.Black
func (svgp *SvgPath) GetFillColor() color.Color {
	return getColor(svgp.fillerColor)
}

// GetLineColor returns the stroke color of the SvgPath if one is defined and otherwise returns colornames.Black
func (svgp *SvgPath) GetLineColor() color.Color {
	return getColor(svgp.linerColor)
}

// SetFillColor sets the fill color of the SvgPath
func (svgp *SvgPath) SetFillColor(clr color.Color) {
	svgp.fillerColor = clr
}

// SetLineColor sets the line color of the SvgPath
func (svgp *SvgPath) SetLineColor(clr color.Color) {
	svgp.linerColor = clr
}

// ParseSVGColorNum reads the SFG color string e.g. #FBD9BD
func ParseSVGColorNum(colorStr string) (r, g, b uint8, err error) {
	colorStr = strings.TrimPrefix(colorStr, "#")
	var t uint64
	if len(colorStr) != 6 {
		// SVG specs say duplicate characters in case of 3 digit hex number
		colorStr = string([]byte{colorStr[0], colorStr[0],
			colorStr[1], colorStr[1], colorStr[2], colorStr[2]})
	}
	for _, v := range []struct {
		c *uint8
		s string
	}{
		{&r, colorStr[0:2]},
		{&g, colorStr[2:4]},
		{&b, colorStr[4:6]}} {
		t, err = strconv.ParseUint(v.s, 16, 8)
		if err != nil {
			return
		}
		*v.c = uint8(t)
	}
	return
}

// ParseSVGColor parses an SVG color string in all forms
// including all SVG1.1 names, obtained from the colornames package
func ParseSVGColor(colorStr string) (color.Color, error) {
	//_, _, _, a := curColor.RGBA()
	v := strings.ToLower(colorStr)
	if strings.HasPrefix(v, "url") { // We are not handling urls
		// and gradients and stuff at this point
		return color.NRGBA{0, 0, 0, 255}, nil
	}
	switch v {
	case "none", "":
		// nil signals that the function (fill or stroke) is off;
		// not the same as black
		return nil, nil
	default:
		cn, ok := colornames.Map[v]
		if ok {
			r, g, b, a := cn.RGBA()
			return color.NRGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, nil
		}
	}
	cStr := strings.TrimPrefix(colorStr, "rgb(")
	if cStr != colorStr {
		cStr := strings.TrimSuffix(cStr, ")")
		vals := strings.Split(cStr, ",")
		if len(vals) != 3 {
			return color.NRGBA{}, errParamMismatch
		}
		var cvals [3]uint8
		var err error
		for i := range cvals {
			cvals[i], err = parseColorValue(vals[i])
			if err != nil {
				return nil, err
			}
		}
		return color.NRGBA{cvals[0], cvals[1], cvals[2], 0xFF}, nil
	}

	cStr = strings.TrimPrefix(colorStr, "hsl(")
	if cStr != colorStr {
		cStr := strings.TrimSuffix(cStr, ")")
		vals := strings.Split(cStr, ",")
		if len(vals) != 3 {
			return color.NRGBA{}, errParamMismatch
		}

		H, err := strconv.ParseInt(strings.TrimSpace(vals[0]), 10, 64)
		if err != nil {
			return color.NRGBA{}, fmt.Errorf("invalid hue in hsl: '%s' (%s)", vals[0], err)
		}

		S, err := strconv.ParseFloat(strings.TrimSpace(vals[1][:len(vals[1])-1]), 64)
		if err != nil {
			return color.NRGBA{}, fmt.Errorf("invalid saturation in hsl: '%s' (%s)", vals[1], err)
		}
		S = S / 100

		L, err := strconv.ParseFloat(strings.TrimSpace(vals[2][:len(vals[2])-1]), 64)
		if err != nil {
			return color.NRGBA{}, fmt.Errorf("invalid lightness in hsl: '%s' (%s)", vals[2], err)
		}
		L = L / 100

		C := (1 - math.Abs((2*L)-1)) * S
		X := C * (1 - math.Abs(math.Mod((float64(H)/60), 2)-1))
		m := L - C/2

		var rp, gp, bp float64
		if H < 60 {
			rp, gp, bp = float64(C), float64(X), float64(0)
		} else if H < 120 {
			rp, gp, bp = float64(X), float64(C), float64(0)
		} else if H < 180 {
			rp, gp, bp = float64(0), float64(C), float64(X)
		} else if H < 240 {
			rp, gp, bp = float64(0), float64(X), float64(C)
		} else if H < 300 {
			rp, gp, bp = float64(X), float64(0), float64(C)
		} else {
			rp, gp, bp = float64(C), float64(0), float64(X)
		}

		r, g, b := math.Round((rp+m)*255), math.Round((gp+m)*255), math.Round((bp+m)*255)
		if r > 255 {
			r = 255
		}
		if g > 255 {
			g = 255
		}
		if b > 255 {
			b = 255
		}

		return color.NRGBA{
			uint8(r),
			uint8(g),
			uint8(b),
			0xFF,
		}, nil
	}

	if colorStr[0] == '#' {
		r, g, b, err := ParseSVGColorNum(colorStr)
		if err != nil {
			return nil, err
		}
		return color.NRGBA{r, g, b, 0xFF}, nil
	}
	return nil, errParamMismatch
}

func parseColorValue(v string) (uint8, error) {
	if v[len(v)-1] == '%' {
		n, err := strconv.Atoi(strings.TrimSpace(v[:len(v)-1]))
		if err != nil {
			return 0, err
		}
		return uint8(n * 0xFF / 100), nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if n > 255 {
		n = 255
	}
	return uint8(n), err
}

func (c *IconCursor) readTransformAttr(m1 rasterx.Matrix2D, k string) (rasterx.Matrix2D, error) {
	ln := len(c.points)
	switch k {
	case "rotate":
		if ln == 1 {
			m1 = m1.Rotate(c.points[0] * math.Pi / 180)
		} else if ln == 3 {
			m1 = m1.Translate(c.points[1], c.points[2]).
				Rotate(c.points[0]*math.Pi/180).
				Translate(-c.points[1], -c.points[2])
		} else {
			return m1, errParamMismatch
		}
	case "translate":
		if ln == 1 {
			m1 = m1.Translate(c.points[0], 0)
		} else if ln == 2 {
			m1 = m1.Translate(c.points[0], c.points[1])
		} else {
			return m1, errParamMismatch
		}
	case "skewx":
		if ln == 1 {
			m1 = m1.SkewX(c.points[0] * math.Pi / 180)
		} else {
			return m1, errParamMismatch
		}
	case "skewy":
		if ln == 1 {
			m1 = m1.SkewY(c.points[0] * math.Pi / 180)
		} else {
			return m1, errParamMismatch
		}
	case "scale":
		if ln == 1 {
			m1 = m1.Scale(c.points[0], 0)
		} else if ln == 2 {
			m1 = m1.Scale(c.points[0], c.points[1])
		} else {
			return m1, errParamMismatch
		}
	case "matrix":
		if ln == 6 {
			m1 = m1.Mult(rasterx.Matrix2D{
				A: c.points[0],
				B: c.points[1],
				C: c.points[2],
				D: c.points[3],
				E: c.points[4],
				F: c.points[5]})
		} else {
			return m1, errParamMismatch
		}
	default:
		return m1, errParamMismatch
	}
	return m1, nil
}

func (c *IconCursor) parseTransform(v string) (rasterx.Matrix2D, error) {
	ts := strings.Split(v, ")")
	m1 := c.StyleStack[len(c.StyleStack)-1].mAdder.M
	for _, t := range ts {
		t = strings.TrimSpace(t)
		if len(t) == 0 {
			continue
		}
		d := strings.Split(t, "(")
		if len(d) != 2 || len(d[1]) < 1 {
			return m1, errParamMismatch // badly formed transformation
		}
		err := c.GetPoints(d[1])
		if err != nil {
			return m1, err
		}
		m1, err = c.readTransformAttr(m1, strings.ToLower(strings.TrimSpace(d[0])))
		if err != nil {
			return m1, err
		}
	}
	return m1, nil
}

func (c *IconCursor) readStyleAttr(curStyle *PathStyle, k, v string) error {
	switch k {
	case "fill":
		gradient, ok := c.ReadGradURL(v, curStyle.fillerColor)
		if ok {
			curStyle.fillerColor = gradient
			break
		}
		var err error
		curStyle.fillerColor, err = ParseSVGColor(v)
		return err
	case "stroke":
		gradient, ok := c.ReadGradURL(v, curStyle.linerColor)
		if ok {
			curStyle.linerColor = gradient
			break
		}
		col, errc := ParseSVGColor(v)
		if errc != nil {
			return errc
		}
		if col != nil {
			curStyle.linerColor = col.(color.NRGBA)
		} else {
			curStyle.linerColor = nil
		}
	case "stroke-linegap":
		switch v {
		case "flat":
			curStyle.LineGap = rasterx.FlatGap
		case "round":
			curStyle.LineGap = rasterx.RoundGap
		case "cubic":
			curStyle.LineGap = rasterx.CubicGap
		case "quadratic":
			curStyle.LineGap = rasterx.QuadraticGap
		}
	case "stroke-leadlinecap":
		switch v {
		case "butt":
			curStyle.LeadLineCap = rasterx.ButtCap
		case "round":
			curStyle.LeadLineCap = rasterx.RoundCap
		case "square":
			curStyle.LeadLineCap = rasterx.SquareCap
		case "cubic":
			curStyle.LeadLineCap = rasterx.CubicCap
		case "quadratic":
			curStyle.LeadLineCap = rasterx.QuadraticCap
		}
	case "stroke-linecap":
		switch v {
		case "butt":
			curStyle.LineCap = rasterx.ButtCap
		case "round":
			curStyle.LineCap = rasterx.RoundCap
		case "square":
			curStyle.LineCap = rasterx.SquareCap
		case "cubic":
			curStyle.LineCap = rasterx.CubicCap
		case "quadratic":
			curStyle.LineCap = rasterx.QuadraticCap
		}
	case "stroke-linejoin":
		switch v {
		case "miter":
			curStyle.LineJoin = rasterx.Miter
		case "miter-clip":
			curStyle.LineJoin = rasterx.MiterClip
		case "arc-clip":
			curStyle.LineJoin = rasterx.ArcClip
		case "round":
			curStyle.LineJoin = rasterx.Round
		case "arc":
			curStyle.LineJoin = rasterx.Arc
		case "bevel":
			curStyle.LineJoin = rasterx.Bevel
		}
	case "stroke-miterlimit":
		mLimit, err := parseFloat(v, 64)
		if err != nil {
			return err
		}
		curStyle.MiterLimit = mLimit
	case "stroke-width":
		width, err := parseFloat(v, 64)
		if err != nil {
			return err
		}
		curStyle.LineWidth = width
	case "stroke-dashoffset":
		dashOffset, err := parseFloat(v, 64)
		if err != nil {
			return err
		}
		curStyle.DashOffset = dashOffset
	case "stroke-dasharray":
		if v != "none" {
			dashes := splitOnCommaOrSpace(v)
			dList := make([]float64, len(dashes))
			for i, dstr := range dashes {
				d, err := parseFloat(strings.TrimSpace(dstr), 64)
				if err != nil {
					return err
				}
				dList[i] = d
			}
			curStyle.Dash = dList
			break
		}
	case "opacity", "stroke-opacity", "fill-opacity":
		op, err := parseFloat(v, 64)
		if err != nil {
			return err
		}
		if k != "stroke-opacity" {
			curStyle.FillOpacity *= op
		}
		if k != "fill-opacity" {
			curStyle.LineOpacity *= op
		}
	case "transform":
		m, err := c.parseTransform(v)
		if err != nil {
			return err
		}
		curStyle.mAdder.M = m
	}
	return nil
}

// PushStyle parses the style element, and push it on the style stack. Only color and opacity are supported
// for fill. Note that this parses both the contents of a style attribute plus
// direct fill and opacity attributes.
func (c *IconCursor) PushStyle(attrs []xml.Attr) error {
	var pairs []string
	className := ""
	for _, attr := range attrs {
		switch strings.ToLower(attr.Name.Local) {
		case "style":
			pairs = append(pairs, strings.Split(attr.Value, ";")...)
		case "class":
			className = attr.Value
		default:
			pairs = append(pairs, attr.Name.Local+":"+attr.Value)
		}
	}
	// Make a copy of the top style
	curStyle := c.StyleStack[len(c.StyleStack)-1]
	for _, pair := range pairs {
		kv := strings.Split(pair, ":")
		if len(kv) >= 2 {
			k := strings.ToLower(kv[0])
			k = strings.TrimSpace(k)
			v := strings.TrimSpace(kv[1])
			err := c.readStyleAttr(&curStyle, k, v)
			if err != nil {
				return err
			}
		}
	}
	c.adaptClasses(&curStyle, className)
	c.StyleStack = append(c.StyleStack, curStyle) // Push style onto stack
	return nil
}

// unitSuffixes are suffixes sometimes applied to the width and height attributes
// of the svg element.
var unitSuffixes = []string{"cm", "mm", "px", "pt"}

// trimSuffixes removes unitSuffixes from any number that is not just numeric
func trimSuffixes(a string) (b string) {
	if a == "" || (a[len(a)-1] >= '0' && a[len(a)-1] <= '9') {
		return a
	}
	b = a
	for _, v := range unitSuffixes {
		b = strings.TrimSuffix(b, v)
	}
	return
}

// parseFloat is a helper function that strips suffixes before passing to strconv.ParseFloat
func parseFloat(s string, bitSize int) (float64, error) {
	val := trimSuffixes(s)
	return strconv.ParseFloat(val, bitSize)
}

// splitOnCommaOrSpace returns a list of strings after splitting the input on comma and space delimiters
func splitOnCommaOrSpace(s string) []string {
	return strings.FieldsFunc(s,
		func(r rune) bool {
			return r == ',' || r == ' '
		})
}

func (c *IconCursor) readStartElement(se xml.StartElement) (err error) {
	var skipDef bool
	if se.Name.Local == "radialGradient" || se.Name.Local == "linearGradient" || c.inGrad {
		skipDef = true
	}
	if c.inDefs && !skipDef {
		ID := ""
		for _, attr := range se.Attr {
			if attr.Name.Local == "id" {
				ID = attr.Value
			}
		}
		if ID != "" && len(c.currentDef) > 0 {
			c.icon.Defs[c.currentDef[0].ID] = c.currentDef
			c.currentDef = make([]definition, 0)
		}
		c.currentDef = append(c.currentDef, definition{
			ID:    ID,
			Tag:   se.Name.Local,
			Attrs: se.Attr,
		})
		return nil
	}
	df, ok := drawFuncs[se.Name.Local]
	if !ok {
		errStr := "Cannot process svg element " + se.Name.Local
		if c.ErrorMode == StrictErrorMode {
			return errors.New(errStr)
		} else if c.ErrorMode == WarnErrorMode {
			log.Println(errStr)
		}
		return nil
	}
	err = df(c, se.Attr)

	if len(c.Path) > 0 {
		//The cursor parsed a path from the xml element
		pathCopy := make(rasterx.Path, len(c.Path))
		copy(pathCopy, c.Path)
		c.icon.SVGPaths = append(c.icon.SVGPaths,
			SvgPath{c.StyleStack[len(c.StyleStack)-1], pathCopy})
		c.Path = c.Path[:0]
	}
	return
}

func (c *IconCursor) adaptClasses(pathStyle *PathStyle, className string) {
	if className == "" || len(c.icon.classes) == 0 {
		return
	}
	for k, v := range c.icon.classes[className] {
		c.readStyleAttr(pathStyle, k, v)
	}
}

// ReadIconStream reads the Icon from the given io.Reader
// This only supports a sub-set of SVG, but
// is enough to draw many icons. If errMode is provided,
// the first value determines if the icon ignores, errors out, or logs a warning
// if it does not handle an element found in the icon file. Ignore warnings is
// the default if no ErrorMode value is provided.
func ReadIconStream(stream io.Reader, errMode ...ErrorMode) (*SvgIcon, error) {
	icon := &SvgIcon{Defs: make(map[string][]definition), Grads: make(map[string]*rasterx.Gradient), Transform: rasterx.Identity}
	cursor := &IconCursor{StyleStack: []PathStyle{DefaultStyle}, icon: icon}
	if len(errMode) > 0 {
		cursor.ErrorMode = errMode[0]
	}
	classInfo := ""
	decoder := xml.NewDecoder(stream)
	decoder.CharsetReader = charset.NewReaderLabel
	for {
		t, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return icon, err
		}
		// Inspect the type of the XML token
		switch se := t.(type) {
		case xml.StartElement:
			// Reads all recognized style attributes from the start element
			// and places it on top of the styleStack
			err = cursor.PushStyle(se.Attr)
			if err != nil {
				return icon, err
			}
			err = cursor.readStartElement(se)
			if err != nil {
				return icon, err
			}
			if se.Name.Local == "style" && cursor.inDefs {
				cursor.inDefsStyle = true
			}
		case xml.EndElement:
			// pop style
			cursor.StyleStack = cursor.StyleStack[:len(cursor.StyleStack)-1]
			switch se.Name.Local {
			case "g":
				if cursor.inDefs {
					cursor.currentDef = append(cursor.currentDef, definition{
						Tag: "endg",
					})
				}
			case "title":
				cursor.inTitleText = false
			case "desc":
				cursor.inDescText = false
			case "defs":
				if len(cursor.currentDef) > 0 {
					cursor.icon.Defs[cursor.currentDef[0].ID] = cursor.currentDef
					cursor.currentDef = make([]definition, 0)
				}
				cursor.inDefs = false
			case "radialGradient", "linearGradient":
				cursor.inGrad = false

			case "style":
				if cursor.inDefsStyle {
					icon.classes, err = parseClasses(classInfo)
					if err != nil {
						return icon, err
					}
					cursor.inDefsStyle = false
				}
			}
		case xml.CharData:
			if cursor.inTitleText {
				icon.Titles[len(icon.Titles)-1] += string(se)
			}
			if cursor.inDescText {
				icon.Descriptions[len(icon.Descriptions)-1] += string(se)
			}
			if cursor.inDefsStyle {
				classInfo = string(se)
			}
		}
	}
	return icon, nil
}

func parseClasses(data string) (map[string]styleAttribute, error) {
	res := map[string]styleAttribute{}
	arr := strings.Split(data, "}")
	for _, v := range arr {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		valueIndex := strings.Index(v, "{")
		if valueIndex == -1 || valueIndex == len(v)-1 {
			return res, errors.New(v + "}: invalid map format in class definitions")
		}
		classesStr := v[:valueIndex]
		attrStr := v[valueIndex+1:]
		attrMap, err := parseAttrs(attrStr)
		if err != nil {
			return res, err
		}
		classes := strings.Split(classesStr, ",")
		for _, class := range classes {
			class = strings.TrimSpace(class)
			if len(class) > 0 && class[0] == '.' {
				class = class[1:]
			}
			for attrKey, attrVal := range attrMap {
				if res[class] == nil {
					res[class] = make(styleAttribute, len(attrMap))
				}
				res[class][attrKey] = attrVal
			}
		}
	}
	return res, nil
}

func parseAttrs(attrStr string) (styleAttribute, error) {
	arr := strings.Split(attrStr, ";")
	res := make(styleAttribute, len(arr))
	for _, kv := range arr {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		tmp := strings.SplitN(kv, ":", 2)
		if len(tmp) != 2 {
			return res, errors.New(kv + ": invalid attribute format")
		}
		k := strings.TrimSpace(tmp[0])
		v := strings.TrimSpace(tmp[1])
		res[k] = v
	}
	return res, nil
}

// ReadIcon reads the Icon from the named file
// This only supports a sub-set of SVG, but
// is enough to draw many icons. If errMode is provided,
// the first value determines if the icon ignores, errors out, or logs a warning
// if it does not handle an element found in the icon file. Ignore warnings is
// the default if no ErrorMode value is provided.
func ReadIcon(iconFile string, errMode ...ErrorMode) (*SvgIcon, error) {
	fin, errf := os.Open(iconFile)
	if errf != nil {
		return nil, errf
	}
	defer fin.Close()
	return ReadIconStream(fin, errMode...)
}

func readFraction(v string) (f float64, err error) {
	v = strings.TrimSpace(v)
	d := 1.0
	if strings.HasSuffix(v, "%") {
		d = 100
		v = strings.TrimSuffix(v, "%")
	}
	f, err = parseFloat(v, 64)
	f /= d
	// Is this is an unnecessary restriction? For now fractions can be all values not just in the range [0,1]
	// if f > 1 {
	// 	f = 1
	// } else if f < 0 {
	// 	f = 0
	// }
	return
}

// getColor is a helper function to get the background color
// if ReadGradUrl needs it.
func getColor(clr interface{}) color.Color {
	switch c := clr.(type) {
	case rasterx.Gradient: // This is a bit lazy but oh well
		for _, s := range c.Stops {
			if s.StopColor != nil {
				return s.StopColor
			}
		}
	case color.NRGBA:
		return c
	}
	return colornames.Black
}

func localizeGradIfStopClrNil(g *rasterx.Gradient, defaultColor interface{}) (grad rasterx.Gradient) {
	grad = *g
	for _, s := range grad.Stops {
		if s.StopColor == nil { // This means we need copy the gradient's Stop slice
			// and fill in the default color

			// Copy the stops
			stops := make([]rasterx.GradStop, len(grad.Stops))
			copy(stops, grad.Stops)
			grad.Stops = stops
			// Use the background color when a stop color is nil
			clr := getColor(defaultColor)
			for i, s := range stops {
				if s.StopColor == nil {
					grad.Stops[i].StopColor = clr
				}
			}
			break // Only need to do this once
		}
	}
	return
}

// ReadGradURL reads an SVG format gradient url
// Since the context of the gradient can affect the colors
// the current fill or line color is passed in and used in
// the case of a nil stopClor value
func (c *IconCursor) ReadGradURL(v string, defaultColor interface{}) (grad rasterx.Gradient, ok bool) {
	if strings.HasPrefix(v, "url(") && strings.HasSuffix(v, ")") {
		urlStr := strings.TrimSpace(v[4 : len(v)-1])
		if strings.HasPrefix(urlStr, "#") {
			var g *rasterx.Gradient
			g, ok = c.icon.Grads[urlStr[1:]]
			if ok {
				grad = localizeGradIfStopClrNil(g, defaultColor)
			}
		}
	}
	return
}

// ReadGradAttr reads an SVG gradient attribute
func (c *IconCursor) ReadGradAttr(attr xml.Attr) (err error) {
	switch attr.Name.Local {
	case "gradientTransform":
		c.grad.Matrix, err = c.parseTransform(attr.Value)
	case "gradientUnits":
		switch strings.TrimSpace(attr.Value) {
		case "userSpaceOnUse":
			c.grad.Units = rasterx.UserSpaceOnUse
		case "objectBoundingBox":
			c.grad.Units = rasterx.ObjectBoundingBox
		}
	case "spreadMethod":
		switch strings.TrimSpace(attr.Value) {
		case "pad":
			c.grad.Spread = rasterx.PadSpread
		case "reflect":
			c.grad.Spread = rasterx.ReflectSpread
		case "repeat":
			c.grad.Spread = rasterx.RepeatSpread
		}
	}
	return
}

type svgFunc func(c *IconCursor, attrs []xml.Attr) error

var (
	drawFuncs = map[string]svgFunc{
		"svg":            svgF,
		"g":              gF,
		"line":           lineF,
		"stop":           stopF,
		"rect":           rectF,
		"circle":         circleF,
		"ellipse":        circleF, //circleF handles ellipse also
		"polyline":       polylineF,
		"polygon":        polygonF,
		"path":           pathF,
		"desc":           descF,
		"defs":           defsF,
		"title":          titleF,
		"linearGradient": linearGradientF,
		"radialGradient": radialGradientF,
	}

	svgF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.icon.ViewBox.X = 0
		c.icon.ViewBox.Y = 0
		c.icon.ViewBox.W = 0
		c.icon.ViewBox.H = 0
		var width, height float64
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "viewBox":
				err = c.GetPoints(attr.Value)
				if len(c.points) != 4 {
					return errParamMismatch
				}
				c.icon.ViewBox.X = c.points[0]
				c.icon.ViewBox.Y = c.points[1]
				c.icon.ViewBox.W = c.points[2]
				c.icon.ViewBox.H = c.points[3]
			case "width":
				width, err = parseFloat(attr.Value, 64)
			case "height":
				height, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		if c.icon.ViewBox.W == 0 {
			c.icon.ViewBox.W = width
		}
		if c.icon.ViewBox.H == 0 {
			c.icon.ViewBox.H = height
		}
		return nil
	}
	gF    svgFunc = func(*IconCursor, []xml.Attr) error { return nil } // g does nothing but push the style
	rectF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var x, y, w, h, rx, ry float64
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "x":
				x, err = parseFloat(attr.Value, 64)
			case "y":
				y, err = parseFloat(attr.Value, 64)
			case "width":
				w, err = parseFloat(attr.Value, 64)
			case "height":
				h, err = parseFloat(attr.Value, 64)
			case "rx":
				rx, err = parseFloat(attr.Value, 64)
			case "ry":
				ry, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		if w == 0 || h == 0 {
			return nil
		}
		rasterx.AddRoundRect(x+c.curX, y+c.curY, w+x+c.curX, h+y+c.curY, rx, ry, 0, rasterx.RoundGap, &c.Path)
		return nil
	}
	circleF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var cx, cy, rx, ry float64
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "cx":
				cx, err = parseFloat(attr.Value, 64)
			case "cy":
				cy, err = parseFloat(attr.Value, 64)
			case "r":
				rx, err = parseFloat(attr.Value, 64)
				ry = rx
			case "rx":
				rx, err = parseFloat(attr.Value, 64)
			case "ry":
				ry, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		if rx == 0 || ry == 0 { // not drawn, but not an error
			return nil
		}
		c.EllipseAt(cx+c.curX, cy+c.curY, rx, ry)
		return nil
	}
	lineF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var x1, x2, y1, y2 float64
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "x1":
				x1, err = parseFloat(attr.Value, 64)
			case "x2":
				x2, err = parseFloat(attr.Value, 64)
			case "y1":
				y1, err = parseFloat(attr.Value, 64)
			case "y2":
				y2, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		c.Path.Start(fixed.Point26_6{
			X: fixed.Int26_6((x1 + c.curX) * 64),
			Y: fixed.Int26_6((y1 + c.curY) * 64)})
		c.Path.Line(fixed.Point26_6{
			X: fixed.Int26_6((x2 + c.curX) * 64),
			Y: fixed.Int26_6((y2 + c.curY) * 64)})
		return nil
	}
	polylineF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "points":
				err = c.GetPoints(attr.Value)
				if len(c.points)%2 != 0 {
					return errors.New("polygon has odd number of points")
				}
			}
			if err != nil {
				return err
			}
		}
		if len(c.points) > 4 {
			c.Path.Start(fixed.Point26_6{
				X: fixed.Int26_6((c.points[0] + c.curX) * 64),
				Y: fixed.Int26_6((c.points[1] + c.curY) * 64)})
			for i := 2; i < len(c.points)-1; i += 2 {
				c.Path.Line(fixed.Point26_6{
					X: fixed.Int26_6((c.points[i] + c.curX) * 64),
					Y: fixed.Int26_6((c.points[i+1] + c.curY) * 64)})
			}
		}
		return nil
	}
	polygonF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		err := polylineF(c, attrs)
		if len(c.points) > 4 {
			c.Path.Stop(true)
		}
		return err
	}
	pathF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "d":
				err = c.CompilePath(attr.Value)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	descF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inDescText = true
		c.icon.Descriptions = append(c.icon.Descriptions, "")
		return nil
	}
	titleF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inTitleText = true
		c.icon.Titles = append(c.icon.Titles, "")
		return nil
	}
	defsF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inDefs = true
		return nil
	}
	linearGradientF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var err error
		c.inGrad = true
		c.grad = &rasterx.Gradient{Points: [5]float64{0, 0, 1, 0, 0},
			IsRadial: false, Bounds: c.icon.ViewBox, Matrix: rasterx.Identity}
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "id":
				id := attr.Value
				if len(id) >= 0 {
					c.icon.Grads[id] = c.grad
				} else {
					return errZeroLengthID
				}
			case "x1":
				c.grad.Points[0], err = readFraction(attr.Value)
			case "y1":
				c.grad.Points[1], err = readFraction(attr.Value)
			case "x2":
				c.grad.Points[2], err = readFraction(attr.Value)
			case "y2":
				c.grad.Points[3], err = readFraction(attr.Value)
			default:
				err = c.ReadGradAttr(attr)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	radialGradientF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inGrad = true
		c.grad = &rasterx.Gradient{Points: [5]float64{0.5, 0.5, 0.5, 0.5, 0.5},
			IsRadial: true, Bounds: c.icon.ViewBox, Matrix: rasterx.Identity}
		var setFx, setFy bool
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "id":
				id := attr.Value
				if len(id) >= 0 {
					c.icon.Grads[id] = c.grad
				} else {
					return errZeroLengthID
				}
			case "r":
				c.grad.Points[4], err = readFraction(attr.Value)
			case "cx":
				c.grad.Points[0], err = readFraction(attr.Value)
			case "cy":
				c.grad.Points[1], err = readFraction(attr.Value)
			case "fx":
				setFx = true
				c.grad.Points[2], err = readFraction(attr.Value)
			case "fy":
				setFy = true
				c.grad.Points[3], err = readFraction(attr.Value)
			default:
				err = c.ReadGradAttr(attr)
			}
			if err != nil {
				return err
			}
		}
		if !setFx { // set fx to cx by default
			c.grad.Points[2] = c.grad.Points[0]
		}
		if !setFy { // set fy to cy by default
			c.grad.Points[3] = c.grad.Points[1]
		}
		return nil
	}
	stopF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var err error
		if c.inGrad {
			stop := rasterx.GradStop{Opacity: 1.0}
			for _, attr := range attrs {
				switch attr.Name.Local {
				case "offset":
					stop.Offset, err = readFraction(attr.Value)
				case "stop-color":
					//todo: add current color inherit
					stop.StopColor, err = ParseSVGColor(attr.Value)
				case "stop-opacity":
					stop.Opacity, err = parseFloat(attr.Value, 64)
				}
				if err != nil {
					return err
				}
			}
			c.grad.Stops = append(c.grad.Stops, stop)
		}
		return nil
	}
	useF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var (
			href string
			x, y float64
			err  error
		)
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "href":
				href = attr.Value
			case "x":
				x, err = parseFloat(attr.Value, 64)
			case "y":
				y, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		c.curX, c.curY = x, y
		defer func() {
			c.curX, c.curY = 0, 0
		}()
		if href == "" {
			return errors.New("only use tags with href is supported")
		}
		if !strings.HasPrefix(href, "#") {
			return errors.New("only the ID CSS selector is supported")
		}
		defs, ok := c.icon.Defs[href[1:]]
		if !ok {
			return errors.New("href ID in use statement was not found in saved defs")
		}
		for _, def := range defs {
			if def.Tag == "endg" {
				// pop style
				c.StyleStack = c.StyleStack[:len(c.StyleStack)-1]
				continue
			}
			if err = c.PushStyle(def.Attrs); err != nil {
				return err
			}
			df, ok := drawFuncs[def.Tag]
			if !ok {
				errStr := "Cannot process svg element " + def.Tag
				if c.ErrorMode == StrictErrorMode {
					return errors.New(errStr)
				} else if c.ErrorMode == WarnErrorMode {
					log.Println(errStr)
				}
				return nil
			}
			if err := df(c, def.Attrs); err != nil {
				return err
			}
			if def.Tag != "g" {
				// pop style
				c.StyleStack = c.StyleStack[:len(c.StyleStack)-1]
			}
		}
		return nil
	}
)

func init() {
	// avoids cyclical static declaration
	// called on package initialization
	drawFuncs["use"] = useF
}
