package mssql

import (
	"bytes"
	"io"
	"strconv"
)

type parser struct {
	r          *bytes.Reader
	w          bytes.Buffer
	paramCount int
	paramMax   int
}

func (p *parser) next() (rune, bool) {
	ch, _, err := p.r.ReadRune()
	if err != nil {
		if err != io.EOF {
			panic(err)
		}
		return 0, false
	}
	return ch, true
}

func (p *parser) unread() {
	err := p.r.UnreadRune()
	if err != nil {
		panic(err)
	}
}

func (p *parser) write(ch rune) {
	p.w.WriteRune(ch)
}

type stateFunc func(*parser) stateFunc

func parseParams(query string) (string, int) {
	p := &parser{
		r: bytes.NewReader([]byte(query)),
	}
	state := parseNormal
	for state != nil {
		state = state(p)
	}
	return p.w.String(), p.paramMax
}

func parseNormal(p *parser) stateFunc {
	for {
		ch, ok := p.next()
		if !ok {
			return nil
		}
		if ch == '?' {
			return parseParameter
		} else if ch == '$' || ch == ':' {
			ch2, ok := p.next()
			if !ok {
				p.write(ch)
				return nil
			}
			p.unread()
			if ch2 >= '0' && ch2 <= '9' {
				return parseParameter
			}
		}
		p.write(ch)
		switch ch {
		case '\'':
			return parseQuote
		case '"':
			return parseDoubleQuote
		case '[':
			return parseBracket
		case '-':
			return parseLineComment
		case '/':
			return parseComment
		}
	}
}

func parseParameter(p *parser) stateFunc {
	var paramN int
	var ok bool
	for {
		var ch rune
		ch, ok = p.next()
		if ok && ch >= '0' && ch <= '9' {
			paramN = paramN*10 + int(ch-'0')
		} else {
			break
		}
	}
	if ok {
		p.unread()
	}
	if paramN == 0 {
		p.paramCount++
		paramN = p.paramCount
	}
	if paramN > p.paramMax {
		p.paramMax = paramN
	}
	p.w.WriteString("@p")
	p.w.WriteString(strconv.Itoa(paramN))
	if !ok {
		return nil
	}
	return parseNormal
}

func parseQuote(p *parser) stateFunc {
	for {
		ch, ok := p.next()
		if !ok {
			return nil
		}
		p.write(ch)
		if ch == '\'' {
			return parseNormal
		}
	}
}

func parseDoubleQuote(p *parser) stateFunc {
	for {
		ch, ok := p.next()
		if !ok {
			return nil
		}
		p.write(ch)
		if ch == '"' {
			return parseNormal
		}
	}
}

func parseBracket(p *parser) stateFunc {
	for {
		ch, ok := p.next()
		if !ok {
			return nil
		}
		p.write(ch)
		if ch == ']' {
			ch, ok = p.next()
			if !ok {
				return nil
			}
			if ch != ']' {
				p.unread()
				return parseNormal
			}
			p.write(ch)
		}
	}
}

func parseLineComment(p *parser) stateFunc {
	ch, ok := p.next()
	if !ok {
		return nil
	}
	if ch != '-' {
		p.unread()
		return parseNormal
	}
	p.write(ch)
	for {
		ch, ok = p.next()
		if !ok {
			return nil
		}
		p.write(ch)
		if ch == '\n' {
			return parseNormal
		}
	}
}

func parseComment(p *parser) stateFunc {
	var nested int
	ch, ok := p.next()
	if !ok {
		return nil
	}
	if ch != '*' {
		p.unread()
		return parseNormal
	}
	p.write(ch)
	for {
		ch, ok = p.next()
		if !ok {
			return nil
		}
		p.write(ch)
		for ch == '*' {
			ch, ok = p.next()
			if !ok {
				return nil
			}
			p.write(ch)
			if ch == '/' {
				if nested == 0 {
					return parseNormal
				} else {
					nested--
				}
			}
		}
		for ch == '/' {
			ch, ok = p.next()
			if !ok {
				return nil
			}
			p.write(ch)
			if ch == '*' {
				nested++
			}
		}
	}
}
