// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"html/template"
	"slices"
	"strings"

	"gitea.dev/modules/util"

	"github.com/alecthomas/chroma/v2"
)

// RenderCodeLinesByLexer highlights zero-based lines using the preceding code as lexer context.
func RenderCodeLinesByLexer(lexer chroma.Lexer, code string, lineNumbers []int) map[int]template.HTML {
	return renderCodeLinesByLexer(lexer, code, normalizeLineNumbers(lineNumbers))
}

func normalizeLineNumbers(lineNumbers []int) []int {
	lineNumbers = slices.Clone(lineNumbers)
	slices.Sort(lineNumbers)
	lineNumbers = slices.Compact(lineNumbers)
	return slices.DeleteFunc(lineNumbers, func(line int) bool { return line < 0 })
}

func renderCodeLinesByLexer(lexer chroma.Lexer, code string, lineNumbers []int) map[int]template.HTML {
	lines := make(map[int]template.HTML, len(lineNumbers))
	if len(lineNumbers) == 0 {
		return lines
	}
	output := renderCodeByLexer(lexer, code, func(iterator chroma.Iterator) chroma.Iterator {
		return codeTokensForLines(iterator, lineNumbers)
	})
	for idx, line := range UnsafeSplitHighlightedLines(output) {
		lines[lineNumbers[idx]] = template.HTML(util.UnsafeBytesToString(line))
	}
	return lines
}

func codeTokensForLines(iterator chroma.Iterator, lineNumbers []int) chroma.Iterator {
	line, target := 0, 0
	var pending chroma.Token
	return func() chroma.Token {
		for target < len(lineNumbers) {
			if pending.Value == "" {
				pending = iterator()
				if pending == chroma.EOF {
					return chroma.EOF
				}
			}
			token := pending
			nl := strings.IndexByte(token.Value, '\n')
			if nl >= 0 {
				token.Value = token.Value[:nl+1]
			}
			pending.Value = pending.Value[len(token.Value):]
			selected := line == lineNumbers[target]
			if nl >= 0 {
				line++
				if selected {
					target++
				}
			}
			if selected {
				return token
			}
		}
		return chroma.EOF
	}
}
