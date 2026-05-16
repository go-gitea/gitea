// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package glob

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// Reference: https://github.com/gobwas/glob/blob/master/glob.go

type Glob interface {
	Match(string) bool
}

type globCompiler struct {
	regexpQuestion     bool
	regexpPlus         bool
	superWildcardRight bool
	supportNegative    bool

	separators        []rune
	nonSeparatorChars string
	globPattern       []rune
	regexpPattern     string
	regexp            *regexp.Regexp
	pos               int
	negativeFlip      bool
}

// compileChars compiles character class patterns like [abc] or [!abc]
func (g *globCompiler) compileChars() (string, error) {
	var result strings.Builder
	result.WriteByte('[')
	if g.pos < len(g.globPattern) && g.globPattern[g.pos] == '!' {
		g.pos++
		result.WriteByte('^')
	}

	for g.pos < len(g.globPattern) {
		c := g.globPattern[g.pos]
		g.pos++

		if c == ']' {
			result.WriteByte(']')
			return result.String(), nil
		}

		if c == '\\' {
			if g.pos >= len(g.globPattern) {
				return "", errors.New("unterminated character class escape")
			}
			result.WriteByte('\\')
			result.WriteRune(g.globPattern[g.pos])
			g.pos++
		} else {
			result.WriteRune(c)
		}
	}

	return "", errors.New("unterminated character class")
}

// compile compiles the glob pattern into a regular expression
func (g *globCompiler) compile(subPattern bool) (string, error) {
	var result strings.Builder

	for g.pos < len(g.globPattern) {
		c := g.globPattern[g.pos]
		g.pos++

		if subPattern && c == '}' {
			var wrapped strings.Builder
			wrapped.Grow(result.Len() + 2)
			wrapped.WriteByte('(')
			wrapped.WriteString(result.String())
			wrapped.WriteByte(')')
			return wrapped.String(), nil
		}

		switch c {
		case '*':
			if g.pos < len(g.globPattern) && g.globPattern[g.pos] == '*' {
				var matchRightSep bool
				if g.superWildcardRight {
					// check "**/" pattern, then the wildcards should also match the right separator
					// e.g.: "**/docs" should match "docs"
					var rightRune rune
					if g.pos+1 < len(g.globPattern) {
						rightRune = g.globPattern[g.pos+1]
					}
					if slices.Contains(g.separators, rightRune) {
						matchRightSep = g.pos-2 < 0 || g.globPattern[g.pos-2] == rightRune
					}
				}
				if matchRightSep {
					g.pos += 2
				} else {
					g.pos++
				}
				result.WriteString(".*") // match any sequence of characters
			} else {
				result.WriteString(g.nonSeparatorChars)
				result.WriteByte('*') // match any sequence of non-separator characters
			}
		case '?':
			if g.regexpQuestion {
				result.WriteByte('?')
			} else {
				result.WriteString(g.nonSeparatorChars) // match any single non-separator character
			}
		case '+':
			if g.regexpPlus {
				result.WriteByte('+')
			} else {
				result.WriteByte('\\')
				result.WriteRune(c)
			}
		case '[':
			chars, err := g.compileChars()
			if err != nil {
				return "", err
			}
			result.WriteString(chars)
		case '{':
			subResult, err := g.compile(true)
			if err != nil {
				return "", err
			}
			result.WriteString(subResult)
		case ',':
			if subPattern {
				result.WriteByte('|')
			} else {
				result.WriteByte(',')
			}
		case '\\':
			if g.pos >= len(g.globPattern) {
				return "", errors.New("no character to escape")
			}
			result.WriteByte('\\')
			result.WriteRune(g.globPattern[g.pos])
			g.pos++
		case '.', '^', '$', '(', ')', '|':
			result.WriteByte('\\')
			result.WriteRune(c) // escape regexp special characters
		default:
			result.WriteRune(c)
		}
	}

	return result.String(), nil
}

func initGlobCompiler(g *globCompiler, pattern string, separators []rune) (Glob, error) {
	g.globPattern = []rune(pattern)
	g.separators = separators

	// Escape separators for use in character class
	escapedSeparators := regexp.QuoteMeta(string(separators))
	if escapedSeparators != "" {
		g.nonSeparatorChars = "[^" + escapedSeparators + "]"
	} else {
		g.nonSeparatorChars = "."
	}

	if g.supportNegative && len(g.globPattern) > 0 && g.globPattern[0] == '!' {
		g.negativeFlip = true
		g.pos++
	}

	compiled, err := g.compile(false)
	if err != nil {
		return nil, err
	}

	var regexpPattern strings.Builder
	regexpPattern.Grow(len(compiled) + 2)
	regexpPattern.WriteByte('^')
	regexpPattern.WriteString(compiled)
	regexpPattern.WriteByte('$')
	g.regexpPattern = regexpPattern.String()

	regex, err := regexp.Compile(g.regexpPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regexp: %w", err)
	}

	g.regexp = regex
	return g, nil
}

func (g *globCompiler) Match(s string) bool {
	ret := g.regexp.MatchString(s)
	if g.negativeFlip {
		ret = !ret
	}
	return ret
}

func Compile(pattern string, separators ...rune) (Glob, error) {
	return initGlobCompiler(&globCompiler{}, pattern, separators)
}

func CompileWorkflow(pattern string) (Glob, error) {
	return initGlobCompiler(&globCompiler{
		regexpQuestion:     true,
		regexpPlus:         true,
		superWildcardRight: true,
		supportNegative:    true,
	}, pattern, []rune{'/'})
}

func MustCompile(pattern string, separators ...rune) Glob {
	g, err := Compile(pattern, separators...)
	if err != nil {
		panic(err)
	}
	return g
}

func IsSpecialByte(c byte) bool {
	return c == '*' || c == '?' || c == '\\' || c == '[' || c == ']' || c == '{' || c == '}'
}

// QuoteMeta returns a string that quotes all glob pattern meta characters
// inside the argument text; For example, QuoteMeta(`{foo*}`) returns `\[foo\*\]`.
// Reference: https://github.com/gobwas/glob/blob/master/glob.go
func QuoteMeta(s string) string {
	pos := 0
	for pos < len(s) && !IsSpecialByte(s[pos]) {
		pos++
	}
	if pos == len(s) {
		return s
	}
	b := make([]byte, pos+2*(len(s)-pos))
	copy(b, s[0:pos])
	to := pos
	for ; pos < len(s); pos++ {
		if IsSpecialByte(s[pos]) {
			b[to] = '\\'
			to++
		}
		b[to] = s[pos]
		to++
	}
	return util.UnsafeBytesToString(b[0:to])
}
