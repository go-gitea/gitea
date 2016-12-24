// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package evaluator

import (
	"regexp"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/util/types"
)

const (
	patMatch = iota + 1
	patOne
	patAny
)

// handle escapes and wild cards convert pattern characters and pattern types,
func compilePattern(pattern string, escape byte) (patChars, patTypes []byte) {
	var lastAny bool
	patChars = make([]byte, len(pattern))
	patTypes = make([]byte, len(pattern))
	patLen := 0
	for i := 0; i < len(pattern); i++ {
		var tp byte
		var c = pattern[i]
		switch c {
		case escape:
			lastAny = false
			tp = patMatch
			if i < len(pattern)-1 {
				i++
				c = pattern[i]
				if c == escape || c == '_' || c == '%' {
					// valid escape.
				} else {
					// invalid escape, fall back to escape byte
					// mysql will treat escape character as the origin value even
					// the escape sequence is invalid in Go or C.
					// e.g, \m is invalid in Go, but in MySQL we will get "m" for select '\m'.
					// Following case is correct just for escape \, not for others like +.
					// TODO: add more checks for other escapes.
					i--
					c = escape
				}
			}
		case '_':
			lastAny = false
			tp = patOne
		case '%':
			if lastAny {
				continue
			}
			lastAny = true
			tp = patAny
		default:
			lastAny = false
			tp = patMatch
		}
		patChars[patLen] = c
		patTypes[patLen] = tp
		patLen++
	}
	for i := 0; i < patLen-1; i++ {
		if (patTypes[i] == patAny) && (patTypes[i+1] == patOne) {
			patTypes[i] = patOne
			patTypes[i+1] = patAny
		}
	}
	patChars = patChars[:patLen]
	patTypes = patTypes[:patLen]
	return
}

const caseDiff = 'a' - 'A'

func matchByteCI(a, b byte) bool {
	if a == b {
		return true
	}
	if a >= 'a' && a <= 'z' && a-caseDiff == b {
		return true
	}
	return a >= 'A' && a <= 'Z' && a+caseDiff == b
}

func doMatch(str string, patChars, patTypes []byte) bool {
	var sIdx int
	for i := 0; i < len(patChars); i++ {
		switch patTypes[i] {
		case patMatch:
			if sIdx >= len(str) || !matchByteCI(str[sIdx], patChars[i]) {
				return false
			}
			sIdx++
		case patOne:
			sIdx++
			if sIdx > len(str) {
				return false
			}
		case patAny:
			i++
			if i == len(patChars) {
				return true
			}
			for sIdx < len(str) {
				if matchByteCI(patChars[i], str[sIdx]) && doMatch(str[sIdx:], patChars[i:], patTypes[i:]) {
					return true
				}
				sIdx++
			}
			return false
		}
	}
	return sIdx == len(str)
}

func (e *Evaluator) patternLike(p *ast.PatternLikeExpr) bool {
	expr := p.Expr.GetValue()
	if expr == nil {
		p.SetValue(nil)
		return true
	}

	sexpr, err := types.ToString(expr)
	if err != nil {
		e.err = errors.Trace(err)
		return false
	}

	// We need to compile pattern if it has not been compiled or it is not static.
	var needCompile = len(p.PatChars) == 0 || !ast.IsConstant(p.Pattern)
	if needCompile {
		pattern := p.Pattern.GetValue()
		if pattern == nil {
			p.SetValue(nil)
			return true
		}
		spattern, err := types.ToString(pattern)
		if err != nil {
			e.err = errors.Trace(err)
			return false
		}
		p.PatChars, p.PatTypes = compilePattern(spattern, p.Escape)
	}
	match := doMatch(sexpr, p.PatChars, p.PatTypes)
	if p.Not {
		match = !match
	}
	p.SetValue(boolToInt64(match))
	return true
}

func (e *Evaluator) patternRegexp(p *ast.PatternRegexpExpr) bool {
	var sexpr string
	if p.Sexpr != nil {
		sexpr = *p.Sexpr
	} else {
		expr := p.Expr.GetValue()
		if expr == nil {
			p.SetValue(nil)
			return true
		}
		var err error
		sexpr, err = types.ToString(expr)
		if err != nil {
			e.err = errors.Errorf("non-string Expression in LIKE: %v (Value of type %T)", expr, expr)
			return false
		}

		if ast.IsConstant(p.Expr) {
			p.Sexpr = new(string)
			*p.Sexpr = sexpr
		}
	}

	re := p.Re
	if re == nil {
		pattern := p.Pattern.GetValue()
		if pattern == nil {
			p.SetValue(nil)
			return true
		}
		spattern, err := types.ToString(pattern)
		if err != nil {
			e.err = errors.Errorf("non-string pattern in LIKE: %v (Value of type %T)", pattern, pattern)
			return false
		}

		if re, err = regexp.Compile(spattern); err != nil {
			e.err = errors.Trace(err)
			return false
		}

		if ast.IsConstant(p.Pattern) {
			p.Re = re
		}
	}
	match := re.MatchString(sexpr)
	if p.Not {
		match = !match
	}
	p.SetValue(boolToInt64(match))
	return true
}
