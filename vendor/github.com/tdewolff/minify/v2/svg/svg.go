// Package svg minifies SVG1.1 following the specifications at http://www.w3.org/TR/SVG11/.
package svg

import (
	"bytes"
	"io"

	"github.com/tdewolff/minify/v2"
	minifyCSS "github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/buffer"
	"github.com/tdewolff/parse/v2/css"
	"github.com/tdewolff/parse/v2/svg"
	"github.com/tdewolff/parse/v2/xml"
)

var (
	voidBytes     = []byte("/>")
	isBytes       = []byte("=")
	spaceBytes    = []byte(" ")
	cdataEndBytes = []byte("]]>")
	pathBytes     = []byte("<path")
	dBytes        = []byte("d")
	zeroBytes     = []byte("0")
	cssMimeBytes  = []byte("text/css")
	urlBytes      = []byte("url(")
)

////////////////////////////////////////////////////////////////

// DefaultMinifier is the default minifier.
var DefaultMinifier = &Minifier{Decimals: -1}

// Minifier is an SVG minifier.
type Minifier struct {
	Decimals int
}

// Minify minifies SVG data, it reads from r and writes to w.
func Minify(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error {
	return DefaultMinifier.Minify(m, w, r, params)
}

// Minify minifies SVG data, it reads from r and writes to w.
func (o *Minifier) Minify(m *minify.M, w io.Writer, r io.Reader, _ map[string]string) error {
	var tag svg.Hash
	defaultStyleType := cssMimeBytes
	defaultStyleParams := map[string]string(nil)
	defaultInlineStyleParams := map[string]string{"inline": "1"}

	p := NewPathData(o)
	minifyBuffer := buffer.NewWriter(make([]byte, 0, 64))
	attrByteBuffer := make([]byte, 0, 64)

	l := xml.NewLexer(r)
	defer l.Restore()

	tb := NewTokenBuffer(l)
	for {
		t := *tb.Shift()
		switch t.TokenType {
		case xml.ErrorToken:
			if l.Err() == io.EOF {
				return nil
			}
			return l.Err()
		case xml.DOCTYPEToken:
			if len(t.Text) > 0 && t.Text[len(t.Text)-1] == ']' {
				if _, err := w.Write(t.Data); err != nil {
					return err
				}
			}
		case xml.TextToken:
			t.Data = parse.ReplaceMultipleWhitespace(parse.TrimWhitespace(t.Data))
			if tag == svg.Style && len(t.Data) > 0 {
				if err := m.MinifyMimetype(defaultStyleType, w, buffer.NewReader(t.Data), defaultStyleParams); err != nil {
					if err != minify.ErrNotExist {
						return err
					} else if _, err := w.Write(t.Data); err != nil {
						return err
					}
				}
			} else if _, err := w.Write(t.Data); err != nil {
				return err
			}
		case xml.CDATAToken:
			if tag == svg.Style {
				minifyBuffer.Reset()
				if err := m.MinifyMimetype(defaultStyleType, minifyBuffer, buffer.NewReader(t.Text), defaultStyleParams); err == nil {
					t.Data = append(t.Data[:9], minifyBuffer.Bytes()...)
					t.Text = t.Data[9:]
					t.Data = append(t.Data, cdataEndBytes...)
				} else if err != minify.ErrNotExist {
					return err
				}
			}
			var useText bool
			if t.Text, useText = xml.EscapeCDATAVal(&attrByteBuffer, t.Text); useText {
				t.Text = parse.ReplaceMultipleWhitespace(parse.TrimWhitespace(t.Text))
				if _, err := w.Write(t.Text); err != nil {
					return err
				}
			} else if _, err := w.Write(t.Data); err != nil {
				return err
			}
		case xml.StartTagPIToken:
			for {
				if t := *tb.Shift(); t.TokenType == xml.StartTagClosePIToken || t.TokenType == xml.ErrorToken {
					break
				}
			}
		case xml.StartTagToken:
			tag = t.Hash
			if tag == svg.Metadata {
				t.Data = nil
			} else if tag == svg.Line {
				o.shortenLine(tb, &t, p)
			} else if tag == svg.Rect {
				o.shortenRect(tb, &t, p)
			} else if tag == svg.Polygon || tag == svg.Polyline {
				o.shortenPoly(tb, &t, p)
			}
			if t.Data == nil {
				skipTag(tb)
			} else if _, err := w.Write(t.Data); err != nil {
				return err
			}
		case xml.AttributeToken:
			if len(t.AttrVal) == 0 || t.Text == nil { // data is nil when attribute has been removed
				continue
			}

			attr := t.Hash
			val := t.AttrVal
			if n, m := parse.Dimension(val); n+m == len(val) && attr != svg.Version { // TODO: inefficient, temporary measure
				val, _ = o.shortenDimension(val)
			}
			if attr == svg.Xml_Space && bytes.Equal(val, []byte("preserve")) ||
				tag == svg.Svg && (attr == svg.Version && bytes.Equal(val, []byte("1.1")) ||
					attr == svg.X && bytes.Equal(val, []byte("0")) ||
					attr == svg.Y && bytes.Equal(val, []byte("0")) ||
					attr == svg.Width && bytes.Equal(val, []byte("100%")) ||
					attr == svg.Height && bytes.Equal(val, []byte("100%")) ||
					attr == svg.PreserveAspectRatio && bytes.Equal(val, []byte("xMidYMid meet")) ||
					attr == svg.BaseProfile && bytes.Equal(val, []byte("none")) ||
					attr == svg.ContentScriptType && bytes.Equal(val, []byte("application/ecmascript")) ||
					attr == svg.ContentStyleType && bytes.Equal(val, []byte("text/css"))) ||
				tag == svg.Style && attr == svg.Type && bytes.Equal(val, []byte("text/css")) {
				continue
			}

			if _, err := w.Write(spaceBytes); err != nil {
				return err
			}
			if _, err := w.Write(t.Text); err != nil {
				return err
			}
			if _, err := w.Write(isBytes); err != nil {
				return err
			}

			if tag == svg.Svg && attr == svg.ContentStyleType {
				val = minify.Mediatype(val)
				defaultStyleType = val
			} else if attr == svg.Style {
				minifyBuffer.Reset()
				if err := m.MinifyMimetype(defaultStyleType, minifyBuffer, buffer.NewReader(val), defaultInlineStyleParams); err == nil {
					val = minifyBuffer.Bytes()
				} else if err != minify.ErrNotExist {
					return err
				}
			} else if attr == svg.D {
				val = p.ShortenPathData(val)
			} else if attr == svg.ViewBox {
				j := 0
				newVal := val[:0]
				for i := 0; i < 4; i++ {
					if i != 0 {
						if j >= len(val) || val[j] != ' ' && val[j] != ',' {
							newVal = append(newVal, val[j:]...)
							break
						}
						newVal = append(newVal, ' ')
						j++
					}
					if dim, n := o.shortenDimension(val[j:]); n > 0 {
						newVal = append(newVal, dim...)
						j += n
					} else {
						newVal = append(newVal, val[j:]...)
						break
					}
				}
				val = newVal
			} else if colorAttrMap[attr] && len(val) > 0 && (len(val) < 5 || !parse.EqualFold(val[:4], urlBytes)) {
				parse.ToLower(val)
				if val[0] == '#' {
					if name, ok := minifyCSS.ShortenColorHex[string(val)]; ok {
						val = name
					} else if len(val) == 7 && val[1] == val[2] && val[3] == val[4] && val[5] == val[6] {
						val[2] = val[3]
						val[3] = val[5]
						val = val[:4]
					}
				} else if hex, ok := minifyCSS.ShortenColorName[css.ToHash(val)]; ok {
					val = hex
					// } else if len(val) > 5 && bytes.Equal(val[:4], []byte("rgb(")) && val[len(val)-1] == ')' {
					// TODO: handle rgb(x, y, z) and hsl(x, y, z)
				}
			}

			// prefer single or double quotes depending on what occurs more often in value
			val = xml.EscapeAttrVal(&attrByteBuffer, val)
			if _, err := w.Write(val); err != nil {
				return err
			}
		case xml.StartTagCloseToken:
			next := tb.Peek(0)
			skipExtra := false
			if next.TokenType == xml.TextToken && parse.IsAllWhitespace(next.Data) {
				next = tb.Peek(1)
				skipExtra = true
			}
			if next.TokenType == xml.EndTagToken {
				// collapse empty tags to single void tag
				tb.Shift()
				if skipExtra {
					tb.Shift()
				}
				if _, err := w.Write(voidBytes); err != nil {
					return err
				}
			} else {
				if _, err := w.Write(t.Data); err != nil {
					return err
				}
			}
		case xml.StartTagCloseVoidToken:
			tag = 0
			if _, err := w.Write(t.Data); err != nil {
				return err
			}
		case xml.EndTagToken:
			tag = 0
			if len(t.Data) > 3+len(t.Text) {
				t.Data[2+len(t.Text)] = '>'
				t.Data = t.Data[:3+len(t.Text)]
			}
			if _, err := w.Write(t.Data); err != nil {
				return err
			}
		}
	}
}

func (o *Minifier) shortenDimension(b []byte) ([]byte, int) {
	if n, m := parse.Dimension(b); n > 0 {
		unit := b[n : n+m]
		b = minify.Number(b[:n], o.Decimals)
		if len(b) != 1 || b[0] != '0' {
			if m == 2 && unit[0] == 'p' && unit[1] == 'x' {
				unit = nil
			} else if m > 1 { // only percentage is length 1
				parse.ToLower(unit)
			}
			b = append(b, unit...)
		}
		return b, n + m
	}
	return b, 0
}

func (o *Minifier) shortenLine(tb *TokenBuffer, t *Token, p *PathData) {
	x1, y1, x2, y2 := zeroBytes, zeroBytes, zeroBytes, zeroBytes
	if attrs, replacee := tb.Attributes(svg.X1, svg.Y1, svg.X2, svg.Y2); replacee != nil {
		// skip converting to path if any attribute contains dimensions, TODO: convert non-percentage dimensions to px
		for _, attr := range attrs {
			if attr != nil {
				if _, dim := parse.Dimension(attr.AttrVal); dim != 0 {
					return
				}
			}
		}

		if attrs[0] != nil {
			x1 = minify.Number(attrs[0].AttrVal, o.Decimals)
			attrs[0].Text = nil
		}
		if attrs[1] != nil {
			y1 = minify.Number(attrs[1].AttrVal, o.Decimals)
			attrs[1].Text = nil
		}
		if attrs[2] != nil {
			x2 = minify.Number(attrs[2].AttrVal, o.Decimals)
			attrs[2].Text = nil
		}
		if attrs[3] != nil {
			y2 = minify.Number(attrs[3].AttrVal, o.Decimals)
			attrs[3].Text = nil
		}

		d := make([]byte, 0, 5+len(x1)+len(y1)+len(x2)+len(y2))
		d = append(d, 'M')
		d = append(d, x1...)
		d = append(d, ' ')
		d = append(d, y1...)
		d = append(d, 'L')
		d = append(d, x2...)
		d = append(d, ' ')
		d = append(d, y2...)
		d = append(d, 'z')
		d = p.ShortenPathData(d)

		t.Data = pathBytes
		replacee.Text = dBytes
		replacee.AttrVal = d
	}
}

func (o *Minifier) shortenRect(tb *TokenBuffer, t *Token, p *PathData) {
	if attrs, replacee := tb.Attributes(svg.X, svg.Y, svg.Width, svg.Height, svg.Rx, svg.Ry); replacee != nil && attrs[4] == nil && attrs[5] == nil {
		// skip converting to path if any attribute contains dimensions, TODO: convert non-percentage dimensions to px
		for _, attr := range attrs {
			if attr != nil {
				if _, dim := parse.Dimension(attr.AttrVal); dim != 0 {
					return
				}
			}
		}

		x, y, w, h := zeroBytes, zeroBytes, zeroBytes, zeroBytes
		if attrs[0] != nil {
			x = minify.Number(attrs[0].AttrVal, o.Decimals)
			attrs[0].Text = nil
		}
		if attrs[1] != nil {
			y = minify.Number(attrs[1].AttrVal, o.Decimals)
			attrs[1].Text = nil
		}
		if attrs[2] != nil {
			w = minify.Number(attrs[2].AttrVal, o.Decimals)
			attrs[2].Text = nil
		}
		if attrs[3] != nil {
			h = minify.Number(attrs[3].AttrVal, o.Decimals)
			attrs[3].Text = nil
		}
		if len(w) == 0 || w[0] == '0' || len(h) == 0 || h[0] == '0' {
			t.Data = nil
			return
		}

		d := make([]byte, 0, 6+2*len(x)+len(y)+len(w)+len(h))
		d = append(d, 'M')
		d = append(d, x...)
		d = append(d, ' ')
		d = append(d, y...)
		d = append(d, 'h')
		d = append(d, w...)
		d = append(d, 'v')
		d = append(d, h...)
		d = append(d, 'H')
		d = append(d, x...)
		d = append(d, 'z')
		d = p.ShortenPathData(d)

		t.Data = pathBytes
		replacee.Text = dBytes
		replacee.AttrVal = d
	}
}

func (o *Minifier) shortenPoly(tb *TokenBuffer, t *Token, p *PathData) {
	if attrs, replacee := tb.Attributes(svg.Points); replacee != nil && attrs[0] != nil {
		points := attrs[0].AttrVal

		i := 0
		for i < len(points) && !(points[i] == ' ' || points[i] == ',' || points[i] == '\n' || points[i] == '\r' || points[i] == '\t') {
			i++
		}
		for i < len(points) && (points[i] == ' ' || points[i] == ',' || points[i] == '\n' || points[i] == '\r' || points[i] == '\t') {
			i++
		}
		for i < len(points) && !(points[i] == ' ' || points[i] == ',' || points[i] == '\n' || points[i] == '\r' || points[i] == '\t') {
			i++
		}
		endMoveTo := i
		for i < len(points) && (points[i] == ' ' || points[i] == ',' || points[i] == '\n' || points[i] == '\r' || points[i] == '\t') {
			i++
		}
		startLineTo := i

		if i == len(points) {
			return
		}

		d := make([]byte, 0, len(points)+3)
		d = append(d, 'M')
		d = append(d, points[:endMoveTo]...)
		d = append(d, 'L')
		d = append(d, points[startLineTo:]...)
		if t.Hash == svg.Polygon {
			d = append(d, 'z')
		}
		d = p.ShortenPathData(d)

		t.Data = pathBytes
		replacee.Text = dBytes
		replacee.AttrVal = d
	}
}

////////////////////////////////////////////////////////////////

func skipTag(tb *TokenBuffer) {
	level := 0
	for {
		if t := *tb.Shift(); t.TokenType == xml.ErrorToken {
			break
		} else if t.TokenType == xml.EndTagToken || t.TokenType == xml.StartTagCloseVoidToken {
			if level == 0 {
				break
			}
			level--
		} else if t.TokenType == xml.StartTagToken {
			level++
		}
	}
}
