// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name     string
		toEscape string
		want     string
	}{
		{
			"Simplest case - nothing to escape",
			"a/b/c/d",
			"a/b/c/d",
		}, {
			"Prefixed tilde - with normal stuff - should not escape",
			"~/src/go/codenest/codenest",
			"~/src/go/codenest/codenest",
		}, {
			"Typical windows path with spaces - should get doublequote escaped",
			`C:\Program Files\CodeNest v1.13 - I like lots of spaces\codenest`,
			`"C:\\Program Files\\CodeNest v1.13 - I like lots of spaces\\codenest"`,
		}, {
			"Forward-slashed windows path with spaces - should get doublequote escaped",
			"C:/Program Files/CodeNest v1.13 - I like lots of spaces/codenest",
			`"C:/Program Files/CodeNest v1.13 - I like lots of spaces/codenest"`,
		}, {
			"Prefixed tilde - but then a space filled path",
			"~git/CodeNest v1.13/codenest",
			`~git/"CodeNest v1.13/codenest"`,
		}, {
			"Bangs are unfortunately not predictable so need to be singlequoted",
			"C:/Program Files/CodeNest!/codenest",
			`'C:/Program Files/CodeNest!/codenest'`,
		}, {
			"Newlines are just irritating",
			"/home/git/CodeNest\n\nWHY-WOULD-YOU-DO-THIS\n\nCodeNest/codenest",
			"'/home/git/CodeNest\n\nWHY-WOULD-YOU-DO-THIS\n\nCodeNest/codenest'",
		}, {
			"Similarly we should nicely handle multiple single quotes if we have to single-quote",
			"'!''!'''!''!'!'",
			`\''!'\'\''!'\'\'\''!'\'\''!'\''!'\'`,
		}, {
			"Double quote < ...",
			"~/<codenest",
			"~/\"<codenest\"",
		}, {
			"Double quote > ...",
			"~/codenest>",
			"~/\"codenest>\"",
		}, {
			"Double quote and escape $ ...",
			"~/$codenest",
			"~/\"\\$codenest\"",
		}, {
			"Double quote {...",
			"~/{codenest",
			"~/\"{codenest\"",
		}, {
			"Double quote }...",
			"~/codenest}",
			"~/\"codenest}\"",
		}, {
			"Double quote ()...",
			"~/(codenest)",
			"~/\"(codenest)\"",
		}, {
			"Double quote and escape `...",
			"~/codenest`",
			"~/\"codenest\\`\"",
		}, {
			"Double quote leading #",
			"#123",
			`"#123"`,
		}, {
			"Double quotes can handle a number of things without having to escape them but not everything ...",
			"~/<codenest> ${codenest} `codenest` [codenest] (codenest) \"codenest\" \\codenest\\ 'codenest'",
			"~/\"<codenest> \\${codenest} \\`codenest\\` [codenest] (codenest) \\\"codenest\\\" \\\\codenest\\\\ 'codenest'\"",
		}, {
			"Single quotes don't need to escape except for '...",
			"~/<codenest> ${codenest} `codenest` (codenest) !codenest! \"codenest\" \\codenest\\ 'codenest'",
			"~/'<codenest> ${codenest} `codenest` (codenest) !codenest! \"codenest\" \\codenest\\ '\\''codenest'\\'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShellEscape(tt.toEscape))
		})
	}
}
