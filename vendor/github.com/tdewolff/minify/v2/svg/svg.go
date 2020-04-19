// Package svg minifies SVG1.1 following the specifications at http://www.w3.org/TR/SVG11/.
package svg

import (
	"bytes"
	"io"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	minifyXML "github.com/tdewolff/minify/v2/xml"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/buffer"
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

// DEPRECATED: DefaultMinifier is the default minifier.
var DefaultMinifier = &Minifier{}

// Minifier is an SVG minifier.
type Minifier struct {
	Decimals     int // DEPRECATED
	Precision    int // number of significant digits
	newPrecision int // precision for new numbers
}

// Minify minifies SVG data, it reads from r and writes to w.
func Minify(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error {
	return (&Minifier{}).Minify(m, w, r, params)
}

// Minify minifies SVG data, it reads from r and writes to w.
func (o *Minifier) Minify(m *minify.M, w io.Writer, r io.Reader, _ map[string]string) error {
	if o.Decimals != 0 {
		minify.Warning.Println("SVG option `Decimals` is deprecated, using as `Precision` instead. Be aware that `Decimals` meant the number of digits behind the dot while `Precision` means the number of significant digits. Example: 1.23 with `Decimals=1` would give 1.2 but with `Pecision=1` gives 1. The default `Decimals=-1` is now `Precision=0` which prints the whole number.")
	}
	if o.Precision == 0 {
		o.Precision = o.Decimals
	}
	o.newPrecision = o.Precision
	if o.newPrecision <= 0 || 15 < o.newPrecision {
		o.newPrecision = 15 // minimum number of digits a double can represent exactly
	}

	var tag Hash
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
			t.Data = parse.ReplaceMultipleWhitespaceAndEntities(t.Data, minifyXML.EntitiesMap, nil)
			t.Data = parse.TrimWhitespace(t.Data)

			if tag == Style && len(t.Data) > 0 {
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
			if tag == Style {
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
				t.Text = parse.ReplaceMultipleWhitespace(t.Text)
				t.Text = parse.TrimWhitespace(t.Text)

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
			if tag == Metadata {
				t.Data = nil
			} else if tag == Rect {
				o.shortenRect(tb, &t)
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
			if n, m := parse.Dimension(val); n+m == len(val) && attr != Version { // TODO: inefficient, temporary measure
				val, _ = o.shortenDimension(val)
			}
			if attr == Xml_Space && bytes.Equal(val, []byte("preserve")) ||
				tag == Svg && (attr == Version && bytes.Equal(val, []byte("1.1")) ||
					attr == X && bytes.Equal(val, []byte("0")) ||
					attr == Y && bytes.Equal(val, []byte("0")) ||
					attr == Width && bytes.Equal(val, []byte("100%")) ||
					attr == Height && bytes.Equal(val, []byte("100%")) ||
					attr == PreserveAspectRatio && bytes.Equal(val, []byte("xMidYMid meet")) ||
					attr == BaseProfile && bytes.Equal(val, []byte("none")) ||
					attr == ContentScriptType && bytes.Equal(val, []byte("application/ecmascript")) ||
					attr == ContentStyleType && bytes.Equal(val, []byte("text/css"))) ||
				tag == Style && attr == Type && bytes.Equal(val, []byte("text/css")) {
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

			if tag == Svg && attr == ContentStyleType {
				val = minify.Mediatype(val)
				defaultStyleType = val
			} else if attr == Style {
				minifyBuffer.Reset()
				if err := m.MinifyMimetype(defaultStyleType, minifyBuffer, buffer.NewReader(val), defaultInlineStyleParams); err == nil {
					val = minifyBuffer.Bytes()
				} else if err != minify.ErrNotExist {
					return err
				}
			} else if attr == D {
				val = p.ShortenPathData(val)
			} else if attr == ViewBox {
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
					if name, ok := css.ShortenColorHex[string(val)]; ok {
						val = name
					} else if len(val) == 7 && val[1] == val[2] && val[3] == val[4] && val[5] == val[6] {
						val[2] = val[3]
						val[3] = val[5]
						val = val[:4]
					}
				} else if hex, ok := css.ShortenColorName[css.ToHash(val)]; ok {
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

			if tag == ForeignObject {
				if err := printTag(w, tb, tag); err != nil {
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
		b = minify.Number(b[:n], o.Precision)
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

func (o *Minifier) shortenRect(tb *TokenBuffer, t *Token) {
	w, h := zeroBytes, zeroBytes
	attrs := tb.Attributes(Width, Height)
	if attrs[0] != nil {
		n, _ := parse.Dimension(attrs[0].AttrVal)
		w = minify.Number(attrs[0].AttrVal[:n], o.Precision)
	}
	if attrs[1] != nil {
		n, _ := parse.Dimension(attrs[1].AttrVal)
		h = minify.Number(attrs[1].AttrVal[:n], o.Precision)
	}
	if len(w) == 0 || w[0] == '0' || len(h) == 0 || h[0] == '0' {
		t.Data = nil
	}
}

////////////////////////////////////////////////////////////////

func printTag(w io.Writer, tb *TokenBuffer, tag Hash) error {
	level := 0
	for {
		t := *tb.Peek(0)
		if level == 0 && t.Hash == tag && (t.TokenType == xml.EndTagToken || t.TokenType == xml.StartTagCloseVoidToken) {
			break
		}
		switch t.TokenType {
		case xml.ErrorToken:
			break
		case xml.StartTagToken:
			if t.Hash == tag {
				level++
			}
		case xml.StartTagCloseVoidToken:
			if t.Hash == tag {
				if level == 0 {
					break
				}
				level--
			}
		case xml.EndTagToken:
			if t.Hash == tag {
				if level == 0 {
					break
				}
				level--
			}
		}
		if _, err := w.Write(t.Data); err != nil {
			return err
		}
		tb.Shift()
	}
	return nil
}

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
