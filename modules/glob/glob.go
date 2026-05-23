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
	builder           *strings.Builder
	pos               int
	negativeFlip      bool
}

// compileChars compiles character class patterns like [abc] or [!abc]
func (g *globCompiler) compileChars() error {
	g.builder.WriteByte('[')
	if g.pos < len(g.globPattern) && g.globPattern[g.pos] == '!' {
		g.pos++
		g.builder.WriteByte('^')
	}

	for g.pos < len(g.globPattern) {
		c := g.globPattern[g.pos]
		g.pos++

		if c == ']' {
			g.builder.WriteByte(']')
			return nil
		}

		if c == '\\' {
			if g.pos >= len(g.globPattern) {
				return errors.New("unterminated character class escape")
			}
			g.builder.WriteByte('\\')
			g.builder.WriteRune(g.globPattern[g.pos])
			g.pos++
		} else {
			g.builder.WriteRune(c)
		}
	}

	return errors.New("unterminated character class")
}

// compile compiles the glob pattern into a regular expression
func (g *globCompiler) compile(subPattern bool) error {
	for g.pos < len(g.globPattern) {
		c := g.globPattern[g.pos]
		g.pos++

		if subPattern && c == '}' {
			g.builder.WriteByte(')')
			return nil
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
				g.builder.WriteString(".*") // match any sequence of characters
			} else {
				g.builder.WriteString(g.nonSeparatorChars)
				g.builder.WriteByte('*') // match any sequence of non-separator characters
			}
		case '?':
			if g.regexpQuestion {
				g.builder.WriteByte('?')
			} else {
				g.builder.WriteString(g.nonSeparatorChars) // match any single non-separator character
			}
		case '+':
			if g.regexpPlus {
				g.builder.WriteByte('+')
			} else {
				g.builder.WriteByte('\\')
				g.builder.WriteRune(c)
			}
		case '[':
			if err := g.compileChars(); err != nil {
				return err
			}
		case '{':
			g.builder.WriteByte('(')
			if err := g.compile(true); err != nil {
				return err
			}
		case ',':
			if subPattern {
				g.builder.WriteByte('|')
			} else {
				g.builder.WriteByte(',')
			}
		case '\\':
			if g.pos >= len(g.globPattern) {
				return errors.New("no character to escape")
			}
			g.builder.WriteByte('\\')
			g.builder.WriteRune(g.globPattern[g.pos])
			g.pos++
		case '.', '^', '$', '(', ')', '|':
			g.builder.WriteByte('\\')
			g.builder.WriteRune(c) // escape regexp special characters
		default:
			g.builder.WriteRune(c)
		}
	}

	return nil
}

func initGlobCompiler(g *globCompiler, pattern string, separators []rune) (Glob, error) {
	g.globPattern = []rune(pattern)
	g.separators = separators
	g.builder = new(strings.Builder)

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

	g.builder.WriteByte('^')
	if err := g.compile(false); err != nil {
		return nil, err
	}
	g.builder.WriteByte('$')
	g.regexpPattern = g.builder.String()
	g.builder = nil

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
