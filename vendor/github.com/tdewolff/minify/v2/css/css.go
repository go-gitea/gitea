// Package css minifies CSS3 following the specifications at http://www.w3.org/TR/css-syntax-3/.
package css

import (
	"bytes"
	"fmt"
	"io"
	"math"
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
	initialBytes      = []byte("initial")
	importantBytes    = []byte("!important")
	dataSchemeBytes   = []byte("data:")
)

type cssMinifier struct {
	m *minify.M
	w io.Writer
	p *css.Parser
	o *Minifier

	tokenBuffer []Token
}

////////////////////////////////////////////////////////////////

// DEPRECATED: DefaultMinifier is the default minifier.
var DefaultMinifier = &Minifier{}

// Minifier is a CSS minifier.
type Minifier struct {
	KeepCSS2     bool
	Decimals     int // DEPRECATED
	Precision    int // number of significant digits
	newPrecision int // precision for new numbers
}

// Minify minifies CSS data, it reads from r and writes to w.
func Minify(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error {
	return (&Minifier{}).Minify(m, w, r, params)
}

type Token struct {
	css.TokenType
	Data       []byte
	Args       []Token // only filled for functions
	Fun, Ident Hash    // only filled for functions and identifiers respectively
}

func (t Token) String() string {
	if len(t.Args) == 0 {
		return t.TokenType.String() + "(" + string(t.Data) + ")"
	}
	return fmt.Sprint(t.Args)
}

func (a Token) Equal(b Token) bool {
	if a.TokenType == b.TokenType && bytes.Equal(a.Data, b.Data) && len(a.Args) == len(b.Args) {
		for i := 0; i < len(a.Args); i++ {
			if a.Args[i].TokenType != b.Args[i].TokenType || !bytes.Equal(a.Args[i].Data, b.Args[i].Data) {
				return false
			}
		}
		return true
	}
	return false
}

func (t Token) IsZero() bool {
	// as each number is already minified, starting with a zero means it is zero
	return (t.TokenType == css.DimensionToken || t.TokenType == css.PercentageToken || t.TokenType == css.NumberToken) && t.Data[0] == '0'
}

func (t Token) IsLength() bool {
	if t.TokenType == css.DimensionToken {
		return true
	} else if t.TokenType == css.NumberToken && t.Data[0] == '0' {
		return true
	} else if t.TokenType == css.FunctionToken {
		fun := ToHash(t.Data[:len(t.Data)-1])
		if fun == Calc || fun == Min || fun == Max || fun == Clamp || fun == Attr || fun == Var || fun == Env {
			return true
		}
	}
	return false
}

func (t Token) IsLengthPercentage() bool {
	return t.TokenType == css.PercentageToken || t.IsLength()
}

// Minify minifies CSS data, it reads from r and writes to w.
func (o *Minifier) Minify(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error {
	if o.Decimals != 0 {
		minify.Warning.Println("CSS option `Decimals` is deprecated, using as `Precision` instead. Be aware that `Decimals` meant the number of digits behind the dot while `Precision` means the number of significant digits. Example: 1.23 with `Decimals=1` would give 1.2 but with `Pecision=1` gives 1. The default `Decimals=-1` is now `Precision=0` which prints the whole number.")
	}
	if o.Precision == 0 {
		o.Precision = o.Decimals
	}
	o.newPrecision = o.Precision
	if o.newPrecision <= 0 || 15 < o.newPrecision {
		o.newPrecision = 15 // minimum number of digits a double can represent exactly
	}

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
			if ToHash(data[1:]) == Import && len(values) == 2 && values[1].TokenType == css.URLToken {
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

func (c *cssMinifier) parseFunction(values []css.Token) ([]Token, int) {
	i := 1
	level := 0
	args := []Token{}
	for ; i < len(values); i++ {
		tt := values[i].TokenType
		data := values[i].Data
		if tt == css.LeftParenthesisToken {
			level++
		} else if tt == css.RightParenthesisToken {
			if level == 0 {
				i++
				break
			}
			level--
		}
		if tt == css.FunctionToken {
			subArgs, di := c.parseFunction(values[i:])
			h := ToHash(parse.ToLower(parse.Copy(data[:len(data)-1]))) // TODO: use ToHashFold
			args = append(args, Token{tt, data, subArgs, h, 0})
			i += di - 1
		} else {
			var h Hash
			if tt == css.IdentToken {
				h = ToHash(parse.ToLower(parse.Copy(data))) // TODO: use ToHashFold
			}
			args = append(args, Token{tt, data, nil, 0, h})
		}
	}
	return args, i
}

func (c *cssMinifier) parseDeclaration(values []css.Token) []Token {
	// Check if this is a simple list of values separated by whitespace or commas, otherwise we'll not be processing
	prevSep := true
	tokens := c.tokenBuffer[:0]
	for i := 0; i < len(values); i++ {
		tt := values[i].TokenType
		data := values[i].Data
		if tt == css.LeftParenthesisToken || tt == css.LeftBraceToken || tt == css.LeftBracketToken ||
			tt == css.RightParenthesisToken || tt == css.RightBraceToken || tt == css.RightBracketToken {
			return nil
		}

		if !prevSep && tt != css.WhitespaceToken && tt != css.CommaToken && (tt != css.DelimToken || values[i].Data[0] != '/') {
			return nil
		}

		if tt == css.WhitespaceToken || tt == css.CommaToken || tt == css.DelimToken && values[i].Data[0] == '/' {
			if tt != css.WhitespaceToken {
				tokens = append(tokens, Token{tt, data, nil, 0, 0})
			}
			prevSep = true
		} else if tt == css.FunctionToken {
			args, di := c.parseFunction(values[i:])
			h := ToHash(parse.ToLower(parse.Copy(data[:len(data)-1]))) // TODO: use ToHashFold
			tokens = append(tokens, Token{tt, data, args, h, 0})
			prevSep = true
			i += di - 1
		} else {
			var h Hash
			if tt == css.IdentToken {
				h = ToHash(parse.ToLower(parse.Copy(data))) // TODO: use ToHashFold
			}
			tokens = append(tokens, Token{tt, data, nil, 0, h})
			prevSep = tt == css.URLToken
		}
	}
	c.tokenBuffer = tokens // update buffer size for memory reuse
	return tokens
}

func (c *cssMinifier) minifyDeclaration(property []byte, components []css.Token) error {
	if _, err := c.w.Write(property); err != nil {
		return err
	}
	if _, err := c.w.Write(colonBytes); err != nil {
		return err
	}

	if len(components) == 0 {
		return nil
	}

	// Strip !important from the component list, this will be added later separately
	important := false
	if len(components) > 2 && components[len(components)-2].TokenType == css.DelimToken && components[len(components)-2].Data[0] == '!' && ToHash(components[len(components)-1].Data) == Important {
		components = components[:len(components)-2]
		important = true
	}

	prop := ToHash(property)
	values := c.parseDeclaration(components)

	// Do not process complex values (eg. containing blocks or is not alternated between whitespace/commas and flat values
	if values == nil {
		if prop == Filter && len(components) == 11 {
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

	values = c.minifyTokens(prop, values)
	if len(values) > 0 {
		values = c.minifyProperty(prop, values)
	}
	return c.writeDeclaration(values, important)
}

func (c *cssMinifier) writeFunction(args []Token) error {
	for _, arg := range args {
		if _, err := c.w.Write(arg.Data); err != nil {
			return err
		}
		if arg.TokenType == css.FunctionToken {
			if err := c.writeFunction(arg.Args); err != nil {
				return err
			}
			if _, err := c.w.Write([]byte(")")); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *cssMinifier) writeDeclaration(values []Token, important bool) error {
	prevSep := true
	for _, value := range values {
		if !prevSep && value.TokenType != css.CommaToken && (value.TokenType != css.DelimToken || value.Data[0] != '/') {
			if _, err := c.w.Write(spaceBytes); err != nil {
				return err
			}
		}

		if _, err := c.w.Write(value.Data); err != nil {
			return err
		}
		if value.TokenType == css.FunctionToken {
			if err := c.writeFunction(value.Args); err != nil {
				return err
			}
			if _, err := c.w.Write([]byte(")")); err != nil {
				return err
			}
		}

		if value.TokenType == css.CommaToken || value.TokenType == css.DelimToken && value.Data[0] == '/' || value.TokenType == css.FunctionToken || value.TokenType == css.URLToken {
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

func (c *cssMinifier) minifyTokens(prop Hash, values []Token) []Token {
	for i, value := range values {
		tt := value.TokenType
		switch tt {
		case css.NumberToken:
			if prop == Z_Index || prop == Counter_Increment || prop == Counter_Reset || prop == Orphans || prop == Widows {
				break // integers
			}
			if c.o.KeepCSS2 {
				values[i].Data = minify.Decimal(values[i].Data, c.o.Precision) // don't use exponents
			} else {
				values[i].Data = minify.Number(values[i].Data, c.o.Precision)
			}
		case css.PercentageToken:
			n := len(values[i].Data) - 1
			if c.o.KeepCSS2 {
				values[i].Data = minify.Decimal(values[i].Data[:n], c.o.Precision) // don't use exponents
			} else {
				values[i].Data = minify.Number(values[i].Data[:n], c.o.Precision)
			}
			values[i].Data = append(values[i].Data, '%')
		case css.DimensionToken:
			var dim []byte
			values[i], dim = c.minifyDimension(values[i])
			if 1 < len(values[i].Data) && values[i].Data[0] == '0' && optionalZeroDimension[string(dim)] && prop != Flex {
				// cut dimension for zero value, TODO: don't hardcode check for Flex and remove the dimension in minifyDimension
				values[i].Data = values[i].Data[:1]
			}
		case css.StringToken:
			values[i].Data = removeMarkupNewlines(values[i].Data)
		case css.URLToken:
			if 10 < len(values[i].Data) {
				uri := parse.TrimWhitespace(values[i].Data[4 : len(values[i].Data)-1])
				delim := byte('"')
				if 1 < len(uri) && (uri[0] == '\'' || uri[0] == '"') {
					delim = uri[0]
					uri = removeMarkupNewlines(uri)
					uri = uri[1 : len(uri)-1]
				}
				if 4 < len(uri) && parse.EqualFold(uri[:5], dataSchemeBytes) {
					uri = minify.DataURI(c.m, uri)
				}
				if css.IsURLUnquoted(uri) {
					values[i].Data = append(append([]byte("url("), uri...), ')')
				} else {
					values[i].Data = append(append(append([]byte("url("), delim), uri...), delim, ')')
				}
			}
		case css.FunctionToken:
			values[i].Args = c.minifyTokens(prop, values[i].Args)

			fun := values[i].Fun
			args := values[i].Args
			if fun == Rgb || fun == Rgba || fun == Hsl || fun == Hsla {
				valid := true
				vals := []float64{}
				for i, arg := range args {
					numeric := arg.TokenType == css.NumberToken || arg.TokenType == css.PercentageToken
					separator := arg.TokenType == css.CommaToken || i != 5 && arg.TokenType == css.WhitespaceToken || i == 5 && arg.TokenType == css.DelimToken && arg.Data[0] == '/'
					if i%2 == 0 && !numeric || i%2 == 1 && !separator {
						valid = false
					} else if numeric {
						var d float64
						if arg.TokenType == css.PercentageToken {
							d, _ = strconv.ParseFloat(string(arg.Data[:len(arg.Data)-1]), 32) // can never fail
							d /= 100.0
							if d < minify.Epsilon {
								d = 0.0
							} else if 1.0-minify.Epsilon < d {
								d = 1.0
							}
						} else {
							d, _ = strconv.ParseFloat(string(arg.Data), 32) // can never fail
						}
						vals = append(vals, d)
					}
				}
				if !valid {
					break
				}

				a := 1.0
				if len(vals) == 4 {
					if vals[3] < minify.Epsilon {
						values[i] = Token{css.IdentToken, transparentBytes, nil, 0, Transparent}
						break
					} else if 1.0-minify.Epsilon < vals[3] {
						vals = vals[:3]
						values[i].Args = values[i].Args[:len(values[i].Args)-2]
						if fun == Rgba || fun == Hsla {
							values[i].Data = values[i].Data[:len(values[i].Data)-1]
							values[i].Data[len(values[i].Data)-1] = '('
						}
					} else {
						a = vals[3]
					}
				}

				if a == 1.0 && (len(vals) == 3 || len(vals) == 4) { // only minify color if fully opaque
					if fun == Rgb || fun == Rgba {
						for j := 0; j < 3; j++ {
							if args[j*2].TokenType == css.NumberToken {
								vals[j] /= 255.0
								if vals[j] < minify.Epsilon {
									vals[j] = 0.0
								} else if 1.0-minify.Epsilon < vals[j] {
									vals[j] = 1.0
								}
							}
						}
						values[i] = rgbToToken(vals[0], vals[1], vals[2])
						break
					} else if fun == Hsl || fun == Hsla && args[0].TokenType == css.NumberToken && args[2].TokenType == css.PercentageToken && args[4].TokenType == css.PercentageToken {
						for vals[0] < 0.0 {
							vals[0] += 360.0
						}
						for 360.0 <= vals[0] {
							vals[0] -= 360.0
						}
						vals[0] /= 360.0

						r, g, b := css.HSL2RGB(vals[0], vals[1], vals[2])
						values[i] = rgbToToken(r, g, b)
						break
					}
				} else if len(vals) == 4 {
					args[6] = minifyNumberPercentage(args[6])
				}

				if fun == Rgb || fun == Rgba {
					// 0%, 20%, 40%, 60%, 80% and 100% can be represented exactly as, 51, 102, 153, 204, and 255 respectively
					removePercentage := true
					for j := 0; j < 3; j++ {
						if args[j*2].TokenType != css.PercentageToken || 2.0*minify.Epsilon <= math.Mod(vals[j]+minify.Epsilon, 0.2) {
							removePercentage = false
							break
						}
					}
					if removePercentage {
						for j := 0; j < 3; j++ {
							args[j*2].TokenType = css.NumberToken
							if vals[j] < minify.Epsilon {
								args[j*2].Data = []byte("0")
							} else if math.Abs(vals[j]-0.2) < minify.Epsilon {
								args[j*2].Data = []byte("51")
							} else if math.Abs(vals[j]-0.4) < minify.Epsilon {
								args[j*2].Data = []byte("102")
							} else if math.Abs(vals[j]-0.6) < minify.Epsilon {
								args[j*2].Data = []byte("153")
							} else if math.Abs(vals[j]-0.8) < minify.Epsilon {
								args[j*2].Data = []byte("204")
							} else if math.Abs(vals[j]-1.0) < minify.Epsilon {
								args[j*2].Data = []byte("255")
							}
						}
					}
				}
			}
		}
	}
	return values
}

func (c *cssMinifier) minifyProperty(prop Hash, values []Token) []Token {
	switch prop {
	case Font:
		if len(values) > 1 { // must contain atleast font-size and font-family
			// the font-families are separated by commas and are at the end of font
			// get index for last token before font family names
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
				} else if h := values[i].Ident; h == Xx_Small || h == X_Small || h == Small || h == Medium || h == Large || h == X_Large || h == Xx_Large || h == Smaller || h == Larger || h == Inherit || h == Initial || h == Unset {
					// inherit, initial and unset are followed by an IdentToken/StringToken, so must be for font-size
					break
				}
			}

			// font-family minified in place
			values = append(values[:i+1], c.minifyProperty(Font_Family, values[i+1:])...)

			// fix for IE9, IE10, IE11: font name starting with `-` is not recognized
			if values[i+1].Data[0] == '-' {
				v := make([]byte, len(values[i+1].Data)+2)
				v[0] = '\''
				copy(v[1:], values[i+1].Data)
				v[len(v)-1] = '\''
				values[i+1].Data = v
			}

			if i > 0 {
				// line-height
				if i > 1 && values[i-1].TokenType == css.DelimToken && values[i-1].Data[0] == '/' {
					if values[i].Ident == Normal {
						values = append(values[:i-1], values[i+1:]...)
					}
					i -= 2
				}

				// font-size
				i--

				for ; i > -1; i-- {
					if values[i].Ident == Normal {
						values = append(values[:i], values[i+1:]...)
					} else if values[i].Ident == Bold {
						values[i].TokenType = css.NumberToken
						values[i].Data = []byte("700")
					} else if values[i].TokenType == css.NumberToken && bytes.Equal(values[i].Data, []byte("400")) {
						values = append(values[:i], values[i+1:]...)
					}
				}
			}
		}
	case Font_Family:
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
	case Font_Weight:
		if values[0].Ident == Normal {
			values[0].TokenType = css.NumberToken
			values[0].Data = []byte("400")
		} else if values[0].Ident == Bold {
			values[0].TokenType = css.NumberToken
			values[0].Data = []byte("700")
		}
	case Url:
		for i := 0; i < len(values); i++ {
			if values[i].TokenType == css.FunctionToken && len(values[i].Args) == 1 {
				fun := values[i].Fun
				data := values[i].Args[0].Data
				if fun == Local && (data[0] == '\'' || data[0] == '"') {
					if css.IsURLUnquoted(data[1 : len(data)-1]) {
						data = data[1 : len(data)-1]
					}
					values[i].Args[0].Data = data
				}
			}
		}
	case Margin, Padding, Border_Width:
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
	case Border, Border_Bottom, Border_Left, Border_Right, Border_Top:
		for i := 0; i < len(values); i++ {
			if values[i].Ident == None || values[i].Ident == Currentcolor || values[i].Ident == Medium {
				values = append(values[:i], values[i+1:]...)
				i--
			} else {
				values[i] = minifyColor(values[i])
			}
		}
		if len(values) == 0 {
			values = []Token{{css.IdentToken, []byte("none"), nil, 0, None}}
		}
	case Outline:
		for i := 0; i < len(values); i++ {
			if values[i].Ident == Invert || values[i].Ident == None || values[i].Ident == Medium {
				values = append(values[:i], values[i+1:]...)
				i--
			} else {
				values[i] = minifyColor(values[i])
			}
		}
		if len(values) == 0 {
			values = []Token{{css.IdentToken, []byte("none"), nil, 0, None}}
		}
	case Background:
		// minify background-size and lowercase all identifiers
		for i := 0; i < len(values); i++ {
			if values[i].TokenType == css.DelimToken && values[i].Data[0] == '/' {
				// background-size consists of either [<length-percentage> | auto | cover | contain] or [<length-percentage> | auto]{2}
				// we can only minify the latter
				if i+1 < len(values) && (values[i+1].TokenType == css.NumberToken || values[i+1].IsLengthPercentage() || values[i+1].Ident == Auto) {
					if i+2 < len(values) && (values[i+2].TokenType == css.NumberToken || values[i+2].IsLengthPercentage() || values[i+2].Ident == Auto) {
						sizeValues := c.minifyProperty(Background_Size, values[i+1:i+3])
						if len(sizeValues) == 1 && sizeValues[0].Ident == Auto {
							// remove background-size if it is '/ auto' after minifying the property
							values = append(values[:i], values[i+3:]...)
							i--
						} else {
							values = append(values[:i+1], append(sizeValues, values[i+3:]...)...)
							i += len(sizeValues) - 1
						}
					} else if values[i+1].Ident == Auto {
						// remove background-size if it is '/ auto'
						values = append(values[:i], values[i+2:]...)
						i--
					}
				}
			}
		}

		// minify all other values
		iPaddingBox := -1 // position of background-origin that is padding-box
		for i := 0; i < len(values); i++ {
			h := values[i].Ident
			values[i] = minifyColor(values[i])
			if values[i].TokenType == css.IdentToken {
				if i+1 < len(values) && values[i+1].TokenType == css.IdentToken && (h == Space || h == Round || h == Repeat || h == No_Repeat) {
					if h2 := values[i+1].Ident; h2 == Space || h2 == Round || h2 == Repeat || h2 == No_Repeat {
						repeatValues := c.minifyProperty(Background_Repeat, values[i:i+2])
						if len(repeatValues) == 1 && repeatValues[0].Ident == Repeat {
							values = append(values[:i], values[i+2:]...)
							i--
						} else {
							values = append(values[:i], append(repeatValues, values[i+2:]...)...)
							i += len(repeatValues) - 1
						}
						continue
					}
				} else if h == None || h == Scroll || h == Transparent {
					values = append(values[:i], values[i+1:]...)
					i--
					continue
				} else if h == Border_Box || h == Padding_Box {
					if iPaddingBox == -1 && h == Padding_Box { // background-origin
						iPaddingBox = i
					} else if iPaddingBox != -1 && h == Border_Box { // background-clip
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

			// further minify background-position and background-size combination
			if values[i].TokenType == css.NumberToken || values[i].IsLengthPercentage() || h == Left || h == Right || h == Top || h == Bottom || h == Center {
				j := i + 1
				for ; j < len(values); j++ {
					if h := values[j].Ident; h == Left || h == Right || h == Top || h == Bottom || h == Center {
						continue
					} else if values[j].TokenType == css.NumberToken || values[j].IsLengthPercentage() {
						continue
					}
					break
				}

				positionValues := c.minifyProperty(Background_Position, values[i:j])
				hasSize := j < len(values) && values[j].TokenType == css.DelimToken && values[j].Data[0] == '/'
				if !hasSize && len(positionValues) == 2 && positionValues[0].IsZero() && positionValues[1].IsZero() {
					values = append(values[:i], values[j:]...)
					i--
				} else {
					values = append(values[:i], append(positionValues, values[j:]...)...)
					i += len(positionValues) - 1
				}
			}
		}

		if len(values) == 0 {
			values = []Token{{css.NumberToken, []byte("0"), nil, 0, 0}, {css.NumberToken, []byte("0"), nil, 0, 0}}
		}
	case Background_Size:
		if len(values) == 2 && values[1].Ident == Auto {
			values = values[:1]
		}
	case Background_Repeat:
		if len(values) == 2 && values[0].TokenType == css.IdentToken && values[1].TokenType == css.IdentToken {
			if values[0].Ident == values[1].Ident {
				values = values[:1]
			} else if values[0].Ident == Repeat && values[1].Ident == No_Repeat {
				values = values[:1]
				values[0].Data = []byte("repeat-x")
				values[0].Ident = Repeat_X
			} else if values[0].Ident == No_Repeat && values[1].Ident == Repeat {
				values = values[:1]
				values[0].Data = []byte("repeat-y")
				values[0].Ident = Repeat_Y
			}
		}
	case Background_Position:
		if len(values) == 3 || len(values) == 4 {
			// remove zero offsets
			for _, i := range []int{len(values) - 1, 1} {
				if 2 < len(values) && values[i].IsZero() {
					values = append(values[:i], values[i+1:]...)
				}
			}

			j := 1 // position of second set of horizontal/vertical values
			if 2 < len(values) && values[2].TokenType == css.IdentToken {
				j = 2
			}

			b := make([]byte, 0, 4)
			offsets := make([]Token, 2)
			for _, i := range []int{j, 0} {
				if i+1 < len(values) && i+1 != j {
					if values[i+1].TokenType == css.PercentageToken {
						// change right or bottom with percentage offset to left or top respectively
						if values[i].Ident == Right || values[i].Ident == Bottom {
							n, _ := strconvParse.ParseInt(values[i+1].Data[:len(values[i+1].Data)-1])
							b = strconv.AppendInt(b[:0], 100-n, 10)
							b = append(b, '%')
							values[i+1].Data = b
							if values[i].Ident == Right {
								values[i].Data = []byte("left")
								values[i].Ident = Left
							} else {
								values[i].Data = []byte("top")
								values[i].Ident = Top
							}
						}
					}
					if values[i].Ident == Left {
						offsets[0] = values[i+1]
					} else if values[i].Ident == Top {
						offsets[1] = values[i+1]
					}
				} else if values[i].Ident == Left {
					offsets[0] = Token{css.NumberToken, []byte("0"), nil, 0, 0}
				} else if values[i].Ident == Top {
					offsets[1] = Token{css.NumberToken, []byte("0"), nil, 0, 0}
				} else if values[i].Ident == Right {
					offsets[0] = Token{css.PercentageToken, []byte("100%"), nil, 0, 0}
					values[i].Ident = Left
				} else if values[i].Ident == Bottom {
					offsets[1] = Token{css.PercentageToken, []byte("100%"), nil, 0, 0}
					values[i].Ident = Top
				}
			}

			if values[0].Ident == Center || values[j].Ident == Center {
				if values[0].Ident == Left || values[j].Ident == Left {
					offsets = offsets[:1]
				} else if values[0].Ident == Top || values[j].Ident == Top {
					offsets[0] = Token{css.NumberToken, []byte("50%"), nil, 0, 0}
				}
			}

			if offsets[0].Data != nil && (len(offsets) == 1 || offsets[1].Data != nil) {
				values = offsets
			}
		}
		// removing zero offsets in the previous loop might make it eligible for the next loop
		if len(values) == 1 || len(values) == 2 {
			if values[0].Ident == Top || values[0].Ident == Bottom {
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
			// transform keywords to lengths|percentages
			for i := 0; i < len(values); i++ {
				if values[i].TokenType == css.IdentToken {
					if values[i].Ident == Left || values[i].Ident == Top {
						values[i].TokenType = css.NumberToken
						values[i].Data = []byte("0")
						values[i].Ident = 0
					} else if values[i].Ident == Right || values[i].Ident == Bottom {
						values[i].TokenType = css.PercentageToken
						values[i].Data = []byte("100%")
						values[i].Ident = 0
					} else if values[i].Ident == Center {
						if i == 0 {
							values[i].TokenType = css.PercentageToken
							values[i].Data = []byte("50%")
							values[i].Ident = 0
						} else {
							values = values[:1]
						}
					}
				} else if i == 1 && values[i].TokenType == css.PercentageToken && bytes.Equal(values[i].Data, []byte("50%")) {
					values = values[:1]
				} else if values[i].TokenType == css.PercentageToken && values[i].Data[0] == '0' {
					values[i].TokenType = css.NumberToken
					values[i].Data = []byte("0")
					values[i].Ident = 0
				}
			}
		}
	case Box_Shadow:
		if len(values) == 1 && (values[0].Ident == None || values[0].Ident == Initial) {
			values = []Token{{css.NumberToken, []byte("0"), nil, 0, 0}, {css.NumberToken, []byte("0"), nil, 0, 0}}
		} else {
			numbers := []int{}
			for i := 0; i < len(values); i++ {
				if values[i].IsLength() {
					numbers = append(numbers, i)
				}
			}
			if len(numbers) == 4 && values[numbers[3]].IsZero() {
				values = append(values[:numbers[3]], values[numbers[3]+1:]...)
				numbers = numbers[:3]
			}
			if len(numbers) == 3 && values[numbers[2]].IsZero() {
				values = append(values[:numbers[2]], values[numbers[2]+1:]...)
			}
		}
	case Ms_Filter:
		alpha := []byte("progid:DXImageTransform.Microsoft.Alpha(Opacity=")
		if values[0].TokenType == css.StringToken && 2 < len(values[0].Data) && bytes.HasPrefix(values[0].Data[1:len(values[0].Data)-1], alpha) {
			values[0].Data = append(append([]byte{values[0].Data[0]}, []byte("alpha(opacity=")...), values[0].Data[1+len(alpha):]...)
		}
	case Color:
		values[0] = minifyColor(values[0])
	case Background_Color:
		values[0] = minifyColor(values[0])
		if values[0].Ident == Transparent {
			values[0].Data = initialBytes
			values[0].Ident = Initial
		}
	case Border_Color:
		sameValues := true
		for i := range values {
			if values[i].Ident == Currentcolor {
				values[i].Data = initialBytes
				values[i].Ident = Initial
			} else {
				values[i] = minifyColor(values[i])
			}
			if 0 < i && sameValues && !bytes.Equal(values[0].Data, values[i].Data) {
				sameValues = false
			}
		}
		if sameValues {
			values = values[:1]
		}
	case Border_Left_Color, Border_Right_Color, Border_Top_Color, Border_Bottom_Color, Text_Decoration_Color, Text_Emphasis_Color:
		if values[0].Ident == Currentcolor {
			values[0].Data = initialBytes
			values[0].Ident = Initial
		} else {
			values[0] = minifyColor(values[0])
		}
	case Caret_Color, Outline_Color, Fill, Stroke:
		values[0] = minifyColor(values[0])
	case Column_Rule:
		for i := 0; i < len(values); i++ {
			if values[i].Ident == Currentcolor || values[i].Ident == None || values[i].Ident == Medium {
				values = append(values[:i], values[i+1:]...)
				i--
			} else {
				values[i] = minifyColor(values[i])
			}
		}
		if len(values) == 0 {
			values = []Token{{css.IdentToken, []byte("none"), nil, 0, None}}
		}
	case Text_Shadow:
		for i := 0; i < len(values); i++ {
			values[i] = minifyColor(values[i])
		}
	case Text_Decoration:
		for i := 0; i < len(values); i++ {
			if values[i].Ident == Currentcolor || values[i].Ident == None || values[i].Ident == Solid {
				values = append(values[:i], values[i+1:]...)
				i--
			} else {
				values[i] = minifyColor(values[i])
			}
		}
		if len(values) == 0 {
			values = []Token{{css.IdentToken, []byte("none"), nil, 0, None}}
		}
	case Text_Emphasis:
		for i := 0; i < len(values); i++ {
			if values[i].Ident == Currentcolor || values[i].Ident == None {
				values = append(values[:i], values[i+1:]...)
				i--
			} else {
				values[i] = minifyColor(values[i])
			}
		}
		if len(values) == 0 {
			values = []Token{{css.IdentToken, []byte("none"), nil, 0, None}}
		}
	case Flex:
		if len(values) == 1 && values[0].TokenType == css.IdentToken {
			if values[0].Ident == None {
				values = []Token{{css.NumberToken, []byte("0"), nil, 0, 0}, {css.NumberToken, []byte("0"), nil, 0, 0}}
			} else if values[0].Ident == Auto {
				values = []Token{{css.NumberToken, []byte("1"), nil, 0, 0}, {css.NumberToken, []byte("1"), nil, 0, 0}}
			} else if values[0].Ident == Initial {
				values = []Token{{css.NumberToken, []byte("0"), nil, 0, 0}, {css.NumberToken, []byte("1"), nil, 0, 0}}
			}
		} else if len(values) == 2 && values[0].TokenType == css.NumberToken {
			if values[1].TokenType != css.NumberToken && values[1].IsZero() {
				values = values[:1] // remove <flex-basis> if it is zero
			} else if values[1].Ident == Auto {
				values[1].TokenType = css.NumberToken
				values[1].Data = []byte("1") // replace <flex-basis> if it is auto by <flex-shrink>
				values[1].Ident = 0
			}
		} else if len(values) == 3 && values[0].TokenType == css.NumberToken && values[1].TokenType == css.NumberToken {
			if len(values[1].Data) == 1 && values[1].Data[0] == '1' && values[2].IsZero() {
				values = values[:1] // remove <flex-shrink> and <flex-basis> if they are 1 and 0 respectively
			} else if values[2].Ident == Auto {
				values = values[:2] // remove auto to write 2-value syntax of <flex-grow> <flex-shrink>
			} else {
				values[2] = minifyLengthPercentage(values[2])
			}
		}
	case Flex_Basis:
		if values[0].Ident == Initial {
			values[0].Data = []byte("auto")
			values[0].Ident = Auto
		} else {
			values[0] = minifyLengthPercentage(values[0])
		}
	case Order, Flex_Grow:
		if values[0].Ident == Initial {
			values[0].TokenType = css.NumberToken
			values[0].Data = []byte("0")
			values[0].Ident = 0
		}
	case Flex_Shrink:
		if values[0].Ident == Initial {
			values[0].TokenType = css.NumberToken
			values[0].Data = []byte("1")
			values[0].Ident = 0
		}
	}
	return values
}

func minifyColor(value Token) Token {
	data := value.Data
	if value.TokenType == css.IdentToken {
		if hexValue, ok := ShortenColorName[value.Ident]; ok {
			value.TokenType = css.HashToken
			value.Data = hexValue
		}
	} else if value.TokenType == css.HashToken {
		parse.ToLower(data[1:])
		if len(data) == 9 && data[7] == data[8] {
			if data[7] == 'f' {
				data = data[:7]
			} else if data[7] == '0' {
				data = []byte("#0000")
			}
		}
		if ident, ok := ShortenColorHex[string(data)]; ok {
			value.TokenType = css.IdentToken
			data = ident
		} else if len(data) == 7 && data[1] == data[2] && data[3] == data[4] && data[5] == data[6] {
			value.TokenType = css.HashToken
			data[2] = data[3]
			data[3] = data[5]
			data = data[:4]
		} else if len(data) == 9 && data[1] == data[2] && data[3] == data[4] && data[5] == data[6] && data[7] == data[8] {
			// from working draft Color Module Level 4
			value.TokenType = css.HashToken
			data[2] = data[3]
			data[3] = data[5]
			data[4] = data[7]
			data = data[:5]
		}
		value.Data = data
	}
	return value
}

func minifyNumberPercentage(value Token) Token {
	// assumes input already minified
	if value.TokenType == css.PercentageToken && 2 < len(value.Data) && value.Data[len(value.Data)-2] == '0' {
		if len(value.Data) == 4 && (value.Data[3] == '.' || value.Data[3] == '%') {
			value.Data[0] = '1'
			value.Data = value.Data[:1]
		} else {
			value.Data[1] = value.Data[0]
			value.Data[0] = '.'
			value.Data = value.Data[:2]
		}
		value.TokenType = css.NumberToken
	} else if value.TokenType == css.NumberToken && 2 < len(value.Data) && value.Data[0] == '.' && value.Data[1] == '0' {
		if value.Data[2] == '0' {
			value.Data[0] = '.'
			copy(value.Data[1:], value.Data[3:])
			value.Data[len(value.Data)-2] = '%'
			value.Data = value.Data[:len(value.Data)-1]
			value.TokenType = css.PercentageToken
		} else if len(value.Data) == 3 {
			value.Data[0] = value.Data[2]
			value.Data[1] = '%'
			value.Data = value.Data[:2]
			value.TokenType = css.PercentageToken
		}
	}
	return value
}

func minifyLengthPercentage(value Token) Token {
	if value.TokenType != css.NumberToken && value.IsZero() {
		value.TokenType = css.NumberToken
		value.Data = value.Data[:1] // remove dimension for zero value
	}
	return value
}

func (c *cssMinifier) minifyDimension(value Token) (Token, []byte) {
	// TODO: add check for zero value
	var dim []byte
	if value.TokenType == css.DimensionToken {
		n := len(value.Data)
		for 0 < n {
			lower := 'a' <= value.Data[n-1] && value.Data[n-1] <= 'z'
			upper := 'A' <= value.Data[n-1] && value.Data[n-1] <= 'Z'
			if !lower && !upper {
				break
			} else if upper {
				value.Data[n-1] = value.Data[n-1] + ('a' - 'A')
			}
			n--
		}

		num := value.Data[:n]
		if c.o.KeepCSS2 {
			num = minify.Decimal(num, c.o.Precision) // don't use exponents
		} else {
			num = minify.Number(num, c.o.Precision)
		}
		dim = value.Data[n:]
		value.Data = append(num, dim...)
	}
	return value, dim

	// TODO: optimize
	//if value.TokenType == css.DimensionToken {
	//	// TODO: reverse; parse dim not number
	//	n := parse.Number(value.Data)
	//	num := value.Data[:n]
	//	dim = value.Data[n:]
	//	parse.ToLower(dim)

	//	if c.o.KeepCSS2 {
	//		num = minify.Decimal(num, c.o.Precision) // don't use exponents
	//	} else {
	//		num = minify.Number(num, c.o.Precision)
	//	}

	//	// change dimension to compress number
	//	h := ToHash(dim)
	//	if h == Px || h == Pt || h == Pc || h == In || h == Mm || h == Cm || h == Q || h == Deg || h == Grad || h == Rad || h == Turn || h == S || h == Ms || h == Hz || h == Khz || h == Dpi || h == Dpcm || h == Dppx {
	//		d, _ := strconv.ParseFloat(string(num), 64) // can never fail
	//		var dimensions []Hash
	//		var multipliers []float64
	//		switch h {
	//		case Px:
	//			//dimensions = []Hash{In, Cm, Pc, Mm, Pt, Q}
	//			//multipliers = []float64{0.010416666666666667, 0.026458333333333333, 0.0625, 0.26458333333333333, 0.75, 1.0583333333333333}
	//			dimensions = []Hash{In, Pc, Pt}
	//			multipliers = []float64{0.010416666666666667, 0.0625, 0.75}
	//		case Pt:
	//			//dimensions = []Hash{In, Cm, Pc, Mm, Px, Q}
	//			//multipliers = []float64{0.013888888888888889, 0.035277777777777778, 0.083333333333333333, 0.35277777777777778, 1.3333333333333333, 1.4111111111111111}
	//			dimensions = []Hash{In, Pc, Px}
	//			multipliers = []float64{0.013888888888888889, 0.083333333333333333, 1.3333333333333333}
	//		case Pc:
	//			//dimensions = []Hash{In, Cm, Mm, Pt, Px, Q}
	//			//multipliers = []float64{0.16666666666666667, 0.42333333333333333, 4.2333333333333333, 12.0, 16.0, 16.933333333333333}
	//			dimensions = []Hash{In, Pt, Px}
	//			multipliers = []float64{0.16666666666666667, 12.0, 16.0}
	//		case In:
	//			//dimensions = []Hash{Cm, Pc, Mm, Pt, Px, Q}
	//			//multipliers = []float64{2.54, 6.0, 25.4, 72.0, 96.0, 101.6}
	//			dimensions = []Hash{Pc, Pt, Px}
	//			multipliers = []float64{6.0, 72.0, 96.0}
	//		case Cm:
	//			//dimensions = []Hash{In, Pc, Mm, Pt, Px, Q}
	//			//multipliers = []float64{0.39370078740157480, 2.3622047244094488, 10.0, 28.346456692913386, 37.795275590551181, 40.0}
	//			dimensions = []Hash{Mm, Q}
	//			multipliers = []float64{10.0, 40.0}
	//		case Mm:
	//			//dimensions = []Hash{In, Cm, Pc, Pt, Px, Q}
	//			//multipliers = []float64{0.039370078740157480, 0.1, 0.23622047244094488, 2.8346456692913386, 3.7795275590551181, 4.0}
	//			dimensions = []Hash{Cm, Q}
	//			multipliers = []float64{0.1, 4.0}
	//		case Q:
	//			//dimensions = []Hash{In, Cm, Pc, Pt, Px} // Q to mm is never smaller
	//			//multipliers = []float64{0.0098425196850393701, 0.025, 0.059055118110236220, 0.70866141732283465, 0.94488188976377953}
	//			dimensions = []Hash{Cm} // Q to mm is never smaller
	//			multipliers = []float64{0.025}
	//		case Deg:
	//			//dimensions = []Hash{Turn, Rad, Grad}
	//			//multipliers = []float64{0.0027777777777777778, 0.017453292519943296, 1.1111111111111111}
	//			dimensions = []Hash{Turn, Grad}
	//			multipliers = []float64{0.0027777777777777778, 1.1111111111111111}
	//		case Grad:
	//			//dimensions = []Hash{Turn, Rad, Deg}
	//			//multipliers = []float64{0.0025, 0.015707963267948966, 0.9}
	//			dimensions = []Hash{Turn, Deg}
	//			multipliers = []float64{0.0025, 0.9}
	//		case Turn:
	//			//dimensions = []Hash{Rad, Deg, Grad}
	//			//multipliers = []float64{6.2831853071795865, 360.0, 400.0}
	//			dimensions = []Hash{Deg, Grad}
	//			multipliers = []float64{360.0, 400.0}
	//		case Rad:
	//			//dimensions = []Hash{Turn, Deg, Grad}
	//			//multipliers = []float64{0.15915494309189534, 57.295779513082321, 63.661977236758134}
	//		case S:
	//			dimensions = []Hash{Ms}
	//			multipliers = []float64{1000.0}
	//		case Ms:
	//			dimensions = []Hash{S}
	//			multipliers = []float64{0.001}
	//		case Hz:
	//			dimensions = []Hash{Khz}
	//			multipliers = []float64{0.001}
	//		case Khz:
	//			dimensions = []Hash{Hz}
	//			multipliers = []float64{1000.0}
	//		case Dpi:
	//			dimensions = []Hash{Dppx, Dpcm}
	//			multipliers = []float64{0.010416666666666667, 0.39370078740157480}
	//		case Dpcm:
	//			//dimensions = []Hash{Dppx, Dpi}
	//			//multipliers = []float64{0.026458333333333333, 2.54}
	//			dimensions = []Hash{Dpi}
	//			multipliers = []float64{2.54}
	//		case Dppx:
	//			//dimensions = []Hash{Dpcm, Dpi}
	//			//multipliers = []float64{37.795275590551181, 96.0}
	//			dimensions = []Hash{Dpi}
	//			multipliers = []float64{96.0}
	//		}
	//		for i := range dimensions {
	//			if dimensions[i] != h { //&& (d < 1.0) == (multipliers[i] > 1.0) {
	//				b, _ := strconvParse.AppendFloat([]byte{}, d*multipliers[i], -1)
	//				if c.o.KeepCSS2 {
	//					b = minify.Decimal(b, c.o.newPrecision) // don't use exponents
	//				} else {
	//					b = minify.Number(b, c.o.newPrecision)
	//				}
	//				newDim := []byte(dimensions[i].String())
	//				if len(b)+len(newDim) < len(num)+len(dim) {
	//					num = b
	//					dim = newDim
	//				}
	//			}
	//		}
	//	}
	//	value.Data = append(num, dim...)
	//}
	return value, dim
}
