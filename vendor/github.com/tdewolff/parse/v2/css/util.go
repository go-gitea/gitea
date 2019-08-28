package css

import "github.com/tdewolff/parse/v2/buffer"

// IsIdent returns true if the bytes are a valid identifier.
func IsIdent(b []byte) bool {
	l := NewLexer(buffer.NewReader(b))
	l.consumeIdentToken()
	l.r.Restore()
	return l.r.Pos() == len(b)
}

// IsURLUnquoted returns true if the bytes are a valid unquoted URL.
func IsURLUnquoted(b []byte) bool {
	l := NewLexer(buffer.NewReader(b))
	l.consumeUnquotedURL()
	l.r.Restore()
	return l.r.Pos() == len(b)
}

// HSL2RGB converts HSL to RGB with all of range [0,1]
// from http://www.w3.org/TR/css3-color/#hsl-color
func HSL2RGB(h, s, l float64) (float64, float64, float64) {
	m2 := l * (s + 1)
	if l > 0.5 {
		m2 = l + s - l*s
	}
	m1 := l*2 - m2
	return hue2rgb(m1, m2, h+1.0/3.0), hue2rgb(m1, m2, h), hue2rgb(m1, m2, h-1.0/3.0)
}

func hue2rgb(m1, m2, h float64) float64 {
	if h < 0.0 {
		h += 1.0
	}
	if h > 1.0 {
		h -= 1.0
	}
	if h*6.0 < 1.0 {
		return m1 + (m2-m1)*h*6.0
	} else if h*2.0 < 1.0 {
		return m2
	} else if h*3.0 < 2.0 {
		return m1 + (m2-m1)*(2.0/3.0-h)*6.0
	}
	return m1
}
