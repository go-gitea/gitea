// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package glob

import (
	"errors"
	"fmt"
	"regexp"

	"code.gitea.io/gitea/modules/util"
)

// Reference: https://github.com/gobwas/glob/blob/master/glob.go

type Glob interface {
	Match(string) bool
}

type globCompiler struct {
	nonSeparatorChars string
	globPattern       []rune
	regexpPattern     string
	regexp            *regexp.Regexp
	pos               int
}

// compileChars compiles character class patterns like [abc] or [!abc]
func (g *globCompiler) compileChars() (string, error) {
	result := ""
	if g.pos < len(g.globPattern) && g.globPattern[g.pos] == '!' {
		g.pos++
		result += "^"
	}

	for g.pos < len(g.globPattern) {
		c := g.globPattern[g.pos]
		g.pos++

		if c == ']' {
			return "[" + result + "]", nil
		}

		if c == '\\' {
			if g.pos >= len(g.globPattern) {
				return "", errors.New("unterminated character class escape")
			}
			result += "\\" + string(g.globPattern[g.pos])
			g.pos++
		} else {
			result += string(c)
		}
	}

	return "", errors.New("unterminated character class")
}

// compile compiles the glob pattern into a regular expression
func (g *globCompiler) compile(subPattern bool) (string, error) {
	result := ""

	for g.pos < len(g.globPattern) {
		c := g.globPattern[g.pos]
		g.pos++

		if subPattern && c == '}' {
			return "(" + result + ")", nil
		}

		switch c {
		case '*':
			if g.pos < len(g.globPattern) && g.globPattern[g.pos] == '*' {
				g.pos++
				result += ".*" // match any sequence of characters
			} else {
				result += g.nonSeparatorChars + "*" // match any sequence of non-separator characters
			}
		case '?':
			result += g.nonSeparatorChars // match any single non-separator character
		case '[':
			chars, err := g.compileChars()
			if err != nil {
				return "", err
			}
			result += chars
		case '{':
			subResult, err := g.compile(true)
			if err != nil {
				return "", err
			}
			result += subResult
		case ',':
			if subPattern {
				result += "|"
			} else {
				result += ","
			}
		case '\\':
			if g.pos >= len(g.globPattern) {
				return "", errors.New("no character to escape")
			}
			result += "\\" + string(g.globPattern[g.pos])
			g.pos++
		case '.', '+', '^', '$', '(', ')', '|':
			result += "\\" + string(c) // escape regexp special characters
		default:
			result += string(c)
		}
	}

	return result, nil
}

func newGlobCompiler(pattern string, separators ...rune) (Glob, error) {
	g := &globCompiler{globPattern: []rune(pattern)}

	// Escape separators for use in character class
	escapedSeparators := regexp.QuoteMeta(string(separators))
	if escapedSeparators != "" {
		g.nonSeparatorChars = "[^" + escapedSeparators + "]"
	} else {
		g.nonSeparatorChars = "."
	}

	compiled, err := g.compile(false)
	if err != nil {
		return nil, err
	}

	g.regexpPattern = "^" + compiled + "$"

	regex, err := regexp.Compile(g.regexpPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regexp: %w", err)
	}

	g.regexp = regex
	return g, nil
}

func (g *globCompiler) Match(s string) bool {
	return g.regexp.MatchString(s)
}

func Compile(pattern string, separators ...rune) (Glob, error) {
	return newGlobCompiler(pattern, separators...)
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
