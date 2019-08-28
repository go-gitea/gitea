// Package css minifies CSS3 following the specifications at http://www.w3.org/TR/css-syntax-3/.
package css

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
	strconvParse "github.com/tdewolff/parse/v2/strconv"
)

var (
	spaceBytes        = []byte(" ")
	colonBytes        = []byte(":")
	semicolonBytes    = []byte(";")
	commaBytes        = []byte(",")
	leftBracketBytes  = []byte("{")
	rightBracketBytes = []byte("}")
	zeroBytes         = []byte("0")
	transparentBytes  = []byte("transparent")
	importantBytes    = []byte("!important")
)

type cssMinifier struct {
	m *minify.M
	w io.Writer
	p *css.Parser
	o *Minifier

	valuesBuffer []Token
}

////////////////////////////////////////////////////////////////

// DefaultMinifier is the default minifier.
var DefaultMinifier = &Minifier{Decimals: -1, KeepCSS2: false}

// Minifier is a CSS minifier.
type Minifier struct {
	Decimals int
	KeepCSS2 bool
}

// Minify minifies CSS data, it reads from r and writes to w.
func Minify(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error {
	return DefaultMinifier.Minify(m, w, r, params)
}

type Token struct {
	css.TokenType
	Data       []byte
	Components []css.Token // only filled for functions
}

func (t Token) String() string {
	if len(t.Components) == 0 {
		return t.TokenType.String() + "(" + string(t.Data) + ")"
	}
	return fmt.Sprint(t.Components)
}

func (a Token) Equal(b Token) bool {
	if a.TokenType == b.TokenType && bytes.Equal(a.Data, b.Data) && len(a.Components) == len(b.Components) {
		for i := 0; i < len(a.Components); i++ {
			if a.Components[i].TokenType != b.Components[i].TokenType || !bytes.Equal(a.Components[i].Data, b.Components[i].Data) {
				return false
			}
		}
		return true
	}
	return false
}

// Minify minifies CSS data, it reads from r and writes to w.
func (o *Minifier) Minify(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error {
	isInline := params != nil && params["inline"] == "1"
	c := &cssMinifier{
		m: m,
		w: w,
		p: css.NewParser(r, isInline),
		o: o,
	}
	defer c.p.Restore()

	if err := c.minifyGrammar(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (c *cssMinifier) minifyGrammar() error {
	semicolonQueued := false
	for {
		gt, _, data := c.p.Next()
		switch gt {
		case css.ErrorGrammar:
			if _, ok := c.p.Err().(*parse.Error); ok {
				if semicolonQueued {
					if _, err := c.w.Write(semicolonBytes); err != nil {
						return err
					}
				}

				// write out the offending declaration (but save the semicolon)
				vals := c.p.Values()
				if len(vals) > 0 && vals[len(vals)-1].TokenType == css.SemicolonToken {
					vals = vals[:len(vals)-1]
					semicolonQueued = true
				}
				for _, val := range vals {
					if _, err := c.w.Write(val.Data); err != nil {
						return err
					}
				}
				continue
			}
			return c.p.Err()
		case css.EndAtRuleGrammar, css.EndRulesetGrammar:
			if _, err := c.w.Write(rightBracketBytes); err != nil {
				return err
			}
			semicolonQueued = false
			continue
		}

		if semicolonQueued {
			if _, err := c.w.Write(semicolonBytes); err != nil {
				return err
			}
			semicolonQueued = false
		}

		switch gt {
		case css.AtRuleGrammar:
			if _, err := c.w.Write(data); err != nil {
				return err
			}
			values := c.p.Values()
			if css.ToHash(data[1:]) == css.Import && len(values) == 2 && values[1].TokenType == css.URLToken {
				url := values[1].Data
				if url[4] != '"' && url[4] != '\'' {
					url = url[3:]
					url[0] = '"'
					url[len(url)-1] = '"'
				} else {
					url = url[4 : len(url)-1]
				}
				values[1].Data = url
			}
			for _, val := range values {
				if _, err := c.w.Write(val.Data); err != nil {
					return err
				}
			}
			semicolonQueued = true
		case css.BeginAtRuleGrammar:
			if _, err := c.w.Write(data); err != nil {
				return err
			}
			for _, val := range c.p.Values() {
				if _, err := c.w.Write(val.Data); err != nil {
					return err
				}
			}
			if _, err := c.w.Write(leftBracketBytes); err != nil {
				return err
			}
		case css.QualifiedRuleGrammar:
			if err := c.minifySelectors(data, c.p.Values()); err != nil {
				return err
			}
			if _, err := c.w.Write(commaBytes); err != nil {
				return err
			}
		case css.BeginRulesetGrammar:
			if err := c.minifySelectors(data, c.p.Values()); err != nil {
				return err
			}
			if _, err := c.w.Write(leftBracketBytes); err != nil {
				return err
			}
		case css.DeclarationGrammar:
			if _, err := c.w.Write(data); err != nil {
				return err
			}
			if _, err := c.w.Write(colonBytes); err != nil {
				return err
			}
			if err := c.minifyDeclaration(data, c.p.Values()); err != nil {
				return err
			}
			semicolonQueued = true
		case css.CustomPropertyGrammar:
			if _, err := c.w.Write(data); err != nil {
				return err
			}
			if _, err := c.w.Write(colonBytes); err != nil {
				return err
			}
			if _, err := c.w.Write(c.p.Values()[0].Data); err != nil {
				return err
			}
			semicolonQueued = true
		case css.CommentGrammar:
			if len(data) > 5 && data[1] == '*' && data[2] == '!' {
				if _, err := c.w.Write(data[:3]); err != nil {
					return err
				}
				comment := parse.TrimWhitespace(parse.ReplaceMultipleWhitespace(data[3 : len(data)-2]))
				if _, err := c.w.Write(comment); err != nil {
					return err
				}
				if _, err := c.w.Write(data[len(data)-2:]); err != nil {
					return err
				}
			}
		default:
			if _, err := c.w.Write(data); err != nil {
				return err
			}
		}
	}
}

func (c *cssMinifier) minifySelectors(property []byte, values []css.Token) error {
	inAttr := false
	isClass := false
	for _, val := range c.p.Values() {
		if !inAttr {
			if val.TokenType == css.IdentToken {
				if !isClass {
					parse.ToLower(val.Data)
				}
				isClass = false
			} else if val.TokenType == css.DelimToken && val.Data[0] == '.' {
				isClass = true
			} else if val.TokenType == css.LeftBracketToken {
				inAttr = true
			}
		} else {
			if val.TokenType == css.StringToken && len(val.Data) > 2 {
				s := val.Data[1 : len(val.Data)-1]
				if css.IsIdent(s) {
					if _, err := c.w.Write(s); err != nil {
						return err
					}
					continue
				}
			} else if val.TokenType == css.RightBracketToken {
				inAttr = false
			} else if val.TokenType == css.IdentToken && len(val.Data) == 1 && (val.Data[0] == 'i' || val.Data[0] == 'I') {
				if _, err := c.w.Write(spaceBytes); err != nil {
					return err
				}
			}
		}
		if _, err := c.w.Write(val.Data); err != nil {
			return err
		}
	}
	return nil
}

func (c *cssMinifier) minifyDeclaration(property []byte, components []css.Token) error {
	if len(components) == 0 {
		return nil
	}

	// Strip !important from the component list, this will be added later separately
	important := false
	if len(components) > 2 && components[len(components)-2].TokenType == css.DelimToken && components[len(components)-2].Data[0] == '!' && css.ToHash(components[len(components)-1].Data) == css.Important {
		components = components[:len(components)-2]
		important = true
	}

	// Check if this is a simple list of values separated by whitespace or commas, otherwise we'll not be processing
	simple := true
	prevSep := true
	values := c.valuesBuffer[:0]

	for i := 0; i < len(components); i++ {
		comp := components[i]
		tt := comp.TokenType

		if tt == css.LeftParenthesisToken || tt == css.LeftBraceToken || tt == css.LeftBracketToken ||
			tt == css.RightParenthesisToken || tt == css.RightBraceToken || tt == css.RightBracketToken {
			simple = false
			break
		}

		if !prevSep && tt != css.WhitespaceToken && tt != css.CommaToken && (tt != css.DelimToken || comp.Data[0] != '/') {
			simple = false
			break
		}

		if tt == css.WhitespaceToken || tt == css.CommaToken || tt == css.DelimToken && comp.Data[0] == '/' {
			prevSep = true
			if tt != css.WhitespaceToken {
				values = append(values, Token{tt, comp.Data, nil})
			}
		} else if tt == css.FunctionToken {
			prevSep = false
			j := i + 1
			level := 0
			for ; j < len(components); j++ {
				if components[j].TokenType == css.LeftParenthesisToken {
					level++
				} else if components[j].TokenType == css.RightParenthesisToken {
					if level == 0 {
						j++
						break
					}
					level--
				}
			}
			values = append(values, Token{components[i].TokenType, components[i].Data, components[i:j]})
			i = j - 1
		} else {
			prevSep = false
			values = append(values, Token{components[i].TokenType, components[i].Data, nil})
		}
	}
	c.valuesBuffer = values

	prop := css.ToHash(property)
	// Do not process complex values (eg. containing blocks or is not alternated between whitespace/commas and flat values
	if !simple {
		if prop == css.Filter && len(components) == 11 {
			if bytes.Equal(components[0].Data, []byte("progid")) &&
				components[1].TokenType == css.ColonToken &&
				bytes.Equal(components[2].Data, []byte("DXImageTransform")) &&
				components[3].Data[0] == '.' &&
				bytes.Equal(components[4].Data, []byte("Microsoft")) &&
				components[5].Data[0] == '.' &&
				bytes.Equal(components[6].Data, []byte("Alpha(")) &&
				bytes.Equal(parse.ToLower(components[7].Data), []byte("opacity")) &&
				components[8].Data[0] == '=' &&
				components[10].Data[0] == ')' {
				components = components[6:]
				components[0].Data = []byte("alpha(")
			}
		}

		for _, component := range components {
			if _, err := c.w.Write(component.Data); err != nil {
				return err
			}
		}
		if important {
			if _, err := c.w.Write(importantBytes); err != nil {
				return err
			}
		}
		return nil
	}

	for i := range values {
		values[i].TokenType, values[i].Data = c.shortenToken(prop, values[i].TokenType, values[i].Data)
	}
	if len(values) > 0 {
		values = c.minifyProperty(prop, values)
	}

	prevSep = true
	for _, value := range values {
		if !prevSep && value.TokenType != css.CommaToken && (value.TokenType != css.DelimToken || value.Data[0] != '/') {
			if _, err := c.w.Write(spaceBytes); err != nil {
				return err
			}
		}

		if value.TokenType == css.FunctionToken {
			err := c.minifyFunction(value.Components)
			if err != nil {
				return err
			}
		} else if _, err := c.w.Write(value.Data); err != nil {
			return err
		}

		if value.TokenType == css.CommaToken || value.TokenType == css.DelimToken && value.Data[0] == '/' {
			prevSep = true
		} else {
			prevSep = false
		}
	}

	if important {
		if _, err := c.w.Write(importantBytes); err != nil {
			return err
		}
	}
	return nil
}

func (c *cssMinifier) minifyProperty(prop css.Hash, values []Token) []Token {
	switch prop {
	case css.Font:
		if len(values) > 1 {
			// the font-families are separated by commas and are at the end of font
			// get index for the first font-family given
			i := len(values) - 1
			for j, value := range values[2:] {
				if value.TokenType == css.CommaToken {
					i = 2 + j - 1 // identifier before first comma is a font-family
					break
				}
			}
			i--

			// advance i while still at font-families when they contain spaces but no quotes
			for ; i > 0; i-- { // i cannot be 0, font-family must be prepended by font-size
				if values[i-1].TokenType == css.DelimToken && values[i-1].Data[0] == '/' {
					break
				} else if values[i].TokenType != css.IdentToken && values[i].TokenType != css.StringToken {
					break
				} else if values[i].TokenType == css.IdentToken {
					h := css.ToHash(values[i].Data)
					// inherit, initial and unset are followed by an IdentToken/StringToken, so must be for font-size
					if h == css.Xx_Small || h == css.X_Small || h == css.Small || h == css.Medium || h == css.Large || h == css.X_Large || h == css.Xx_Large || h == css.Smaller || h == css.Larger || h == css.Inherit || h == css.Initial || h == css.Unset {
						break
					}
				}
			}

			// font-family minified in place
			values = append(values[:i+1], c.minifyProperty(css.Font_Family, values[i+1:])...)

			if i > 0 {
				// line-height
				if i > 1 && values[i-1].TokenType == css.DelimToken && values[i-1].Data[0] == '/' {
					if values[i].TokenType == css.IdentToken && bytes.Equal(values[i].Data, []byte("normal")) {
						values = append(values[:i-1], values[i+1:]...)
					}
					i -= 2
				}

				// font-size
				i--

				for ; i > -1; i-- {
					if values[i].TokenType == css.IdentToken {
						val := css.ToHash(values[i].Data)
						if val == css.Normal {
							values = append(values[:i], values[i+1:]...)
						} else if val == css.Bold {
							values[i].TokenType = css.NumberToken
							values[i].Data = []byte("700")
						}
					} else if values[i].TokenType == css.NumberToken && bytes.Equal(values[i].Data, []byte("400")) {
						values = append(values[:i], values[i+1:]...)
					}
				}
			}
		}
	case css.Font_Family:
		for i, value := range values {
			if value.TokenType == css.StringToken && len(value.Data) > 2 {
				unquote := true
				parse.ToLower(value.Data)
				s := value.Data[1 : len(value.Data)-1]
				if len(s) > 0 {
					for _, split := range bytes.Split(s, spaceBytes) {
						// if len is zero, it contains two consecutive spaces
						if len(split) == 0 || !css.IsIdent(split) {
							unquote = false
							break
						}
					}
				}
				if unquote {
					values[i].Data = s
				}
			}
		}
	case css.Font_Weight:
		if len(values) == 1 && values[0].TokenType == css.IdentToken {
			val := css.ToHash(values[0].Data)
			if val == css.Normal {
				values[0].TokenType = css.NumberToken
				values[0].Data = []byte("400")
			} else if val == css.Bold {
				values[0].TokenType = css.NumberToken
				values[0].Data = []byte("700")
			}
		}
	case css.Margin, css.Padding, css.Border_Width:
		switch len(values) {
		case 2:
			if values[0].Equal(values[1]) {
				values = values[:1]
			}
		case 3:
			if values[0].Equal(values[1]) && values[0].Equal(values[2]) {
				values = values[:1]
			} else if values[0].Equal(values[2]) {
				values = values[:2]
			}
		case 4:
			if values[0].Equal(values[1]) && values[0].Equal(values[2]) && values[0].Equal(values[3]) {
				values = values[:1]
			} else if values[0].Equal(values[2]) && values[1].Equal(values[3]) {
				values = values[:2]
			} else if values[1].Equal(values[3]) {
				values = values[:3]
			}
		}
	case css.Border, css.Border_Bottom, css.Border_Left, css.Border_Right, css.Border_Top:
		for i := 0; i < len(values); i++ {
			if values[i].TokenType == css.IdentToken {
				val := css.ToHash(values[i].Data)
				if val == css.None || val == css.Currentcolor || val == css.Medium {
					values = append(values[:i], values[i+1:]...)
					i--
				}
			}
		}
		if len(values) == 0 {
			values = []Token{{css.IdentToken, []byte("none"), nil}}
		}
	case css.Outline:
		for i := 0; i < len(values); i++ {
			if values[i].TokenType == css.IdentToken {
				val := css.ToHash(values[i].Data)
				if val == css.None || val == css.Medium { // color=invert is not supported by all browsers
					values = append(values[:i], values[i+1:]...)
					i--
				}
			}
		}
		if len(values) == 0 {
			values = []Token{{css.IdentToken, []byte("none"), nil}}
		}
	case css.Background:
		// TODO: multiple background layers separated by comma
		hasSize := false
		for i := 0; i < len(values); i++ {
			if values[i].TokenType == css.DelimToken && values[i].Data[0] == '/' {
				hasSize = true
				// background-size consists of either [<length-percentage> | auto | cover | contain] or [<length-percentage> | auto]{2}
				// we can only minify the latter
				if i+1 < len(values) && (values[i+1].TokenType == css.NumberToken || values[i+1].TokenType == css.PercentageToken || values[i+1].TokenType == css.IdentToken && bytes.Equal(values[i+1].Data, []byte("auto")) || values[i+1].TokenType == css.FunctionToken) {
					if i+2 < len(values) && (values[i+2].TokenType == css.NumberToken || values[i+2].TokenType == css.PercentageToken || values[i+2].TokenType == css.IdentToken && bytes.Equal(values[i+2].Data, []byte("auto")) || values[i+2].TokenType == css.FunctionToken) {
						sizeValues := c.minifyProperty(css.Background_Size, values[i+1:i+3])
						if len(sizeValues) == 1 && bytes.Equal(sizeValues[0].Data, []byte("auto")) {
							// remove background-size if it is '/ auto' after minifying the property
							values = append(values[:i], values[i+3:]...)
							hasSize = false
							i--
						} else {
							values = append(values[:i+1], append(sizeValues, values[i+3:]...)...)
							i += len(sizeValues) - 1
						}
					} else if values[i+1].TokenType == css.IdentToken && bytes.Equal(values[i+1].Data, []byte("auto")) {
						// remove background-size if it is '/ auto'
						values = append(values[:i], values[i+2:]...)
						hasSize = false
						i--
					}
				}
			}
		}

		var h css.Hash
		iPaddingBox := -1 // position of background-origin that is padding-box
		for i := 0; i < len(values); i++ {
			if values[i].TokenType == css.IdentToken {
				h = css.ToHash(values[i].Data)
				if i+1 < len(values) && values[i+1].TokenType == css.IdentToken && (h == css.Space || h == css.Round || h == css.Repeat || h == css.No_Repeat) {
					if h2 := css.ToHash(values[i+1].Data); h2 == css.Space || h2 == css.Round || h2 == css.Repeat || h2 == css.No_Repeat {
						repeatValues := c.minifyProperty(css.Background_Repeat, values[i:i+2])
						if len(repeatValues) == 1 && bytes.Equal(repeatValues[0].Data, []byte("repeat")) {
							values = append(values[:i], values[i+2:]...)
							i--
						} else {
							values = append(values[:i], append(repeatValues, values[i+2:]...)...)
							i += len(repeatValues) - 1
						}
						continue
					}
				} else if h == css.None || h == css.Scroll || h == css.Transparent {
					values = append(values[:i], values[i+1:]...)
					i--
					continue
				} else if h == css.Border_Box || h == css.Padding_Box {
					if iPaddingBox == -1 && h == css.Padding_Box { // background-origin
						iPaddingBox = i
					} else if iPaddingBox != -1 && h == css.Border_Box { // background-clip
						values = append(values[:i], values[i+1:]...)
						values = append(values[:iPaddingBox], values[iPaddingBox+1:]...)
						i -= 2
					}
					continue
				}
			} else if values[i].TokenType == css.HashToken && bytes.Equal(values[i].Data, []byte("#0000")) {
				values = append(values[:i], values[i+1:]...)
				i--
				continue
			}

			// background-position or background-size
			// TODO: allow only functions that return Number, Percentage or Dimension token? Make whitelist?
			if values[i].TokenType == css.NumberToken || values[i].TokenType == css.DimensionToken || values[i].TokenType == css.PercentageToken || values[i].TokenType == css.IdentToken && (h == css.Left || h == css.Right || h == css.Top || h == css.Bottom || h == css.Center) || values[i].TokenType == css.FunctionToken {
				j := i + 1
				for ; j < len(values); j++ {
					if values[j].TokenType == css.IdentToken {
						h := css.ToHash(values[j].Data)
						if h == css.Left || h == css.Right || h == css.Top || h == css.Bottom || h == css.Center {
							continue
						}
					} else if values[j].TokenType == css.NumberToken || values[j].TokenType == css.DimensionToken || values[j].TokenType == css.PercentageToken || values[j].TokenType == css.FunctionToken {
						continue
					}
					break
				}

				positionValues := c.minifyProperty(css.Background_Position, values[i:j])
				if !hasSize && len(positionValues) == 2 && positionValues[0].TokenType == css.NumberToken && bytes.Equal(positionValues[0].Data, []byte("0")) && positionValues[0].Equal(positionValues[1]) {
					values = append(values[:i], values[j:]...)
					i--
				} else {
					values = append(values[:i], append(positionValues, values[j:]...)...)
					i += len(positionValues) - 1
				}
			}
		}

		if len(values) == 0 {
			values = []Token{{css.NumberToken, []byte("0"), nil}, {css.NumberToken, []byte("0"), nil}}
		}
	case css.Background_Size:
		if len(values) == 2 && values[1].TokenType == css.IdentToken && bytes.Equal(values[1].Data, []byte("auto")) {
			values = values[:1]
		}
	case css.Background_Repeat:
		if len(values) == 2 && values[0].TokenType == css.IdentToken && values[1].TokenType == css.IdentToken {
			h0 := css.ToHash(values[0].Data)
			h1 := css.ToHash(values[1].Data)
			if h0 == h1 {
				values = values[:1]
			} else if h0 == css.Repeat && h1 == css.No_Repeat {
				values = values[:1]
				values[0].Data = []byte("repeat-x")
			} else if h0 == css.No_Repeat && h1 == css.Repeat {
				values = values[:1]
				values[0].Data = []byte("repeat-y")
			}
		}
	case css.Background_Position:
		if len(values) == 3 || len(values) == 4 {
			// remove zero offsets
			for _, i := range []int{len(values) - 1, 1} {
				if values[i].TokenType == css.NumberToken && bytes.Equal(values[i].Data, []byte("0")) || values[i].TokenType == css.PercentageToken && bytes.Equal(values[i].Data, []byte("0%")) {
					values = append(values[:i], values[i+1:]...)
				}
			}

			j := 1 // position of second set of horizontal/vertical values
			if 2 < len(values) && values[2].TokenType == css.IdentToken {
				j = 2
			}
			hs := make([]css.Hash, 3)
			hs[0] = css.ToHash(values[0].Data)
			hs[j] = css.ToHash(values[j].Data)

			b := make([]byte, 0, 4)
			offsets := make([]Token, 2)
			for _, i := range []int{j, 0} {
				if i+1 < len(values) && i+1 != j {
					if values[i+1].TokenType == css.PercentageToken {
						// change right or bottom with percentage offset to left or top respectively
						if hs[i] == css.Right || hs[i] == css.Bottom {
							n, _ := strconvParse.ParseInt(values[i+1].Data[:len(values[i+1].Data)-1])
							b = strconv.AppendInt(b[:0], 100-n, 10)
							b = append(b, '%')
							values[i+1].Data = b
							if hs[i] == css.Right {
								values[i].Data = []byte("left")
								hs[i] = css.Left
							} else {
								values[i].Data = []byte("top")
								hs[i] = css.Top
							}
						}
					}
					if hs[i] == css.Left {
						offsets[0] = values[i+1]
					} else if hs[i] == css.Top {
						offsets[1] = values[i+1]
					}
				} else if hs[i] == css.Left {
					offsets[0] = Token{css.NumberToken, []byte("0"), nil}
				} else if hs[i] == css.Top {
					offsets[1] = Token{css.NumberToken, []byte("0"), nil}
				} else if hs[i] == css.Right {
					offsets[0] = Token{css.PercentageToken, []byte("100%"), nil}
					hs[i] = css.Left
				} else if hs[i] == css.Bottom {
					offsets[1] = Token{css.PercentageToken, []byte("100%"), nil}
					hs[i] = css.Top
				}
			}

			if hs[0] == css.Center || hs[j] == css.Center {
				if hs[0] == css.Left || hs[j] == css.Left {
					offsets = offsets[:1]
				} else if hs[0] == css.Top || hs[j] == css.Top {
					offsets[0] = Token{css.NumberToken, []byte("50%"), nil}
				}
			}

			if offsets[0].Data != nil && (len(offsets) == 1 || offsets[1].Data != nil) {
				values = offsets
			}
		}
		// removing zero offsets in the previous loop might make it eligible for the next loop
		if len(values) == 1 || len(values) == 2 {
			if values[0].TokenType == css.IdentToken {
				h := css.ToHash(values[0].Data)
				if h == css.Top || h == css.Bottom {
					if len(values) == 1 {
						// we can't make this smaller, and converting to a number will break it
						// (https://github.com/tdewolff/minify/issues/221#issuecomment-415419918)
						break
					}
					// if it's a vertical position keyword, swap it with the next element
					// since otherwise converted number positions won't be valid anymore
					// (https://github.com/tdewolff/minify/issues/221#issue-353067229)
					values[0], values[1] = values[1], values[0]
				}
			}
			// transform keywords to lengths|percentages
			for i := 0; i < len(values); i++ {
				if values[i].TokenType == css.IdentToken {
					h := css.ToHash(values[i].Data)
					if h == css.Left || h == css.Top {
						values[i].TokenType = css.NumberToken
						values[i].Data = []byte("0")
					} else if h == css.Right || h == css.Bottom {
						values[i].TokenType = css.PercentageToken
						values[i].Data = []byte("100%")
					} else if h == css.Center {
						if i == 0 {
							values[i].TokenType = css.PercentageToken
							values[i].Data = []byte("50%")
						} else {
							values = values[:1]
						}
					}
				} else if i == 1 && values[i].TokenType == css.PercentageToken && bytes.Equal(values[i].Data, []byte("50%")) {
					values = values[:1]
				} else if values[i].TokenType == css.PercentageToken && bytes.Equal(values[i].Data, []byte("0%")) {
					values[i].TokenType = css.NumberToken
					values[i].Data = []byte("0")
				}
			}
		}
	case css.Box_Shadow:
		if len(values) == 4 && len(values[0].Data) == 1 && values[0].Data[0] == '0' && len(values[1].Data) == 1 && values[1].Data[0] == '0' && len(values[2].Data) == 1 && values[2].Data[0] == '0' && len(values[3].Data) == 1 && values[3].Data[0] == '0' {
			values = values[:2]
		}
	case css.Ms_Filter:
		alpha := []byte("progid:DXImageTransform.Microsoft.Alpha(Opacity=")
		if values[0].TokenType == css.StringToken && bytes.HasPrefix(values[0].Data[1:len(values[0].Data)-1], alpha) {
			values[0].Data = append(append([]byte{values[0].Data[0]}, []byte("alpha(opacity=")...), values[0].Data[1+len(alpha):]...)
		}
	}
	return values
}

func (c *cssMinifier) minifyColorAsHex(rgba [3]byte) error {
	val := make([]byte, 7)
	val[0] = '#'
	hex.Encode(val[1:], rgba[:])
	parse.ToLower(val)
	if s, ok := ShortenColorHex[string(val[:7])]; ok {
		if _, err := c.w.Write(s); err != nil {
			return err
		}
		return nil
	} else if val[1] == val[2] && val[3] == val[4] && val[5] == val[6] {
		val[2] = val[3]
		val[3] = val[5]
		val = val[:4]
	} else {
		val = val[:7]
	}

	_, err := c.w.Write(val)
	return err
}

func (c *cssMinifier) minifyFunction(values []css.Token) error {
	if n := len(values); n > 2 {
		fun := css.ToHash(values[0].Data[0 : len(values[0].Data)-1])
		if fun == css.Rgb || fun == css.Rgba || fun == css.Hsl || fun == css.Hsla {
			valid := true
			vals := make([]*css.Token, 0, 4)
			for i, value := range values[1 : n-1] {
				numeric := value.TokenType == css.NumberToken || value.TokenType == css.PercentageToken
				separator := value.TokenType == css.CommaToken || i != 5 && value.TokenType == css.WhitespaceToken || i == 5 && value.TokenType == css.DelimToken && value.Data[0] == '/'
				if i%2 == 0 && !numeric || i%2 == 1 && !separator {
					valid = false
				} else if numeric {
					vals = append(vals, &values[i+1])
				}
			}

			if valid {
				for _, val := range vals {
					val.TokenType, val.Data = c.shortenToken(0, val.TokenType, val.Data)
				}

				a := byte(255)
				if len(vals) == 4 {
					d, _ := strconv.ParseFloat(string(values[7].Data), 32) // can never fail because if valid == true than this is a NumberToken or PercentageToken
					if d < minify.Epsilon {                                // zero or less
						_, err := c.w.Write(transparentBytes)
						return err
					} else if d >= 1.0 {
						values = values[:7]
					} else {
						a = byte(d*255.0 + 0.5)
					}
				}

				if a == 255 { // only minify color if fully opaque
					if (fun == css.Rgb || fun == css.Rgba) && (len(vals) == 3 || len(vals) == 4) {
						rgb := [3]byte{}

						for j, val := range vals[:3] {
							if val.TokenType == css.NumberToken {
								d, _ := strconv.ParseInt(string(val.Data), 10, 32)
								if d < 0 {
									d = 0
								} else if d > 255 {
									d = 255
								}
								rgb[j] = byte(d)
							} else if val.TokenType == css.PercentageToken {
								d, _ := strconv.ParseFloat(string(val.Data[:len(val.Data)-1]), 32)
								if d < 0.0 {
									d = 0.0
								} else if d > 100.0 {
									d = 100.0
								}
								rgb[j] = byte((d / 100.0 * 255.0) + 0.5)
							}
						}
						return c.minifyColorAsHex(rgb)
					} else if (fun == css.Hsl || fun == css.Hsla) && (len(vals) == 3 || len(vals) == 4) && vals[0].TokenType == css.NumberToken && vals[1].TokenType == css.PercentageToken && vals[2].TokenType == css.PercentageToken {
						h, _ := strconv.ParseFloat(string(vals[0].Data), 32)
						s, _ := strconv.ParseFloat(string(vals[1].Data[:len(vals[1].Data)-1]), 32)
						l, _ := strconv.ParseFloat(string(vals[2].Data[:len(vals[2].Data)-1]), 32)
						for h > 360.0 {
							h -= 360.0
						}
						if s < 0.0 {
							s = 0.0
						} else if s > 100.0 {
							s = 100.0
						}
						if l < 0.0 {
							l = 0.0
						} else if l > 100.0 {
							l = 100.0
						}

						r, g, b := css.HSL2RGB(h/360.0, s/100.0, l/100.0)
						rgb := [3]byte{byte((r * 255.0) + 0.5), byte((g * 255.0) + 0.5), byte((b * 255.0) + 0.5)}
						return c.minifyColorAsHex(rgb)
					}
				}
			}
		} else if fun == css.Local && n == 3 {
			data := values[1].Data
			if data[0] == '\'' || data[0] == '"' {
				data = removeStringNewlinex(data)
				if css.IsURLUnquoted(data[1 : len(data)-1]) {
					data = data[1 : len(data)-1]
				}
				values[1].Data = data
			}
		}
	}

	for _, value := range values {
		if _, err := c.w.Write(value.Data); err != nil {
			return err
		}
	}
	return nil
}

func (c *cssMinifier) shortenToken(prop css.Hash, tt css.TokenType, data []byte) (css.TokenType, []byte) {
	switch tt {
	case css.NumberToken, css.PercentageToken, css.DimensionToken:
		if tt == css.NumberToken && (prop == css.Z_Index || prop == css.Counter_Increment || prop == css.Counter_Reset || prop == css.Orphans || prop == css.Widows) {
			return tt, data // integers
		}
		n := len(data)
		if tt == css.PercentageToken {
			n--
		} else if tt == css.DimensionToken {
			n = parse.Number(data)
		}
		dim := data[n:]
		parse.ToLower(dim)
		if !c.o.KeepCSS2 {
			data = minify.Number(data[:n], c.o.Decimals)
		} else {
			data = minify.Decimal(data[:n], c.o.Decimals) // don't use exponents
		}
		if tt == css.DimensionToken && (len(data) != 1 || data[0] != '0' || !optionalZeroDimension[string(dim)] || prop == css.Flex) {
			data = append(data, dim...)
		} else if tt == css.PercentageToken {
			data = append(data, '%') // TODO: drop percentage for properties that accept <percentage> and <length>
		}
	case css.IdentToken:
		parse.ToLower(parse.Copy(data)) // not all identifiers are case-insensitive; all <custom-ident> properties are case-sensitive
		hash := css.ToHash(data)
		if hexValue, ok := ShortenColorName[hash]; ok {
			tt = css.HashToken
			data = hexValue
		}
	case css.HashToken:
		parse.ToLower(data)
		if len(data) == 9 && data[7] == data[8] {
			if data[7] == 'f' {
				data = data[:7]
			} else if data[7] == '0' {
				data = []byte("#0000")
			}
		}
		if ident, ok := ShortenColorHex[string(data)]; ok {
			tt = css.IdentToken
			data = ident
		} else if len(data) == 7 && data[1] == data[2] && data[3] == data[4] && data[5] == data[6] {
			tt = css.HashToken
			data[2] = data[3]
			data[3] = data[5]
			data = data[:4]
		} else if len(data) == 9 && data[1] == data[2] && data[3] == data[4] && data[5] == data[6] && data[7] == data[8] {
			// from working draft Color Module Level 4
			tt = css.HashToken
			data[2] = data[3]
			data[3] = data[5]
			data[4] = data[7]
			data = data[:5]
		}
	case css.StringToken:
		data = removeStringNewlinex(data)
	case css.URLToken:
		parse.ToLower(data[:3])
		if len(data) > 10 {
			uri := parse.TrimWhitespace(data[4 : len(data)-1])
			delim := byte('"')
			if uri[0] == '\'' || uri[0] == '"' {
				delim = uri[0]
				uri = removeStringNewlinex(uri)
				uri = uri[1 : len(uri)-1]
			}
			uri = minify.DataURI(c.m, uri)
			if css.IsURLUnquoted(uri) {
				data = append(append([]byte("url("), uri...), ')')
			} else {
				data = append(append(append([]byte("url("), delim), uri...), delim, ')')
			}
		}
	}
	return tt, data
}

func removeStringNewlinex(data []byte) []byte {
	// remove any \\\r\n \\\r \\\n
	for i := 1; i < len(data)-2; i++ {
		if data[i] == '\\' && (data[i+1] == '\n' || data[i+1] == '\r') {
			// encountered first replacee, now start to move bytes to the front
			j := i + 2
			if data[i+1] == '\r' && len(data) > i+2 && data[i+2] == '\n' {
				j++
			}
			for ; j < len(data); j++ {
				if data[j] == '\\' && len(data) > j+1 && (data[j+1] == '\n' || data[j+1] == '\r') {
					if data[j+1] == '\r' && len(data) > j+2 && data[j+2] == '\n' {
						j++
					}
					j++
				} else {
					data[i] = data[j]
					i++
				}
			}
			data = data[:i]
			break
		}
	}
	return data
}
