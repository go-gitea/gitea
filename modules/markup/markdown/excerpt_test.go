// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirstExcerptLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "plain", input: "first\nsecond", expected: "first"},
		{name: "leading blank lines", input: "\n\nfirst", expected: "first"},
		{name: "CRLF", input: "```go\r\ncode\r\n```\r\n\r\nfirst\r\nsecond", expected: "first"},
		{name: "inline code", input: "I really like `foo!`, nice job!", expected: "I really like `foo!`, nice job!"},
		{name: "heading", input: "# first", expected: "# first"},
		{name: "syntax-only heading", input: "###\n\nfirst", expected: "first"},
		{name: "tight list", input: "- first", expected: "- first"},
		{name: "loose list", input: "- first\n\n- second", expected: "- first"},
		{name: "blockquote", input: "> first", expected: "> first"},
		{name: "note attention", input: "> [!NOTE]\n> first", expected: "> first"},
		{name: "escaped attention", input: `> \[!NOTE\]` + "\n> first", expected: "> first"},
		{name: "emphasis attention", input: "> **note**\n> first", expected: "> first"},
		{name: "attention separate paragraph", input: "> [!NOTE]\n>\n> first", expected: "> first"},
		{name: "attention only", input: "> [!NOTE]", expected: ""},
		{name: "attention with same-line prose", input: "> [!NOTE] first", expected: "> [!NOTE] first"},
		{name: "unknown attention", input: "> [!UNKNOWN]\n> first", expected: "> [!UNKNOWN]"},
		{name: "unknown escaped attention", input: `> \[!UNKNOWN\]` + "\n> first", expected: `> \[!UNKNOWN\]`},
		{name: "unknown emphasis attention", input: "> **unknown**\n> first", expected: "> **unknown**"},
		{name: "thematic break", input: "---\n\nfirst", expected: "first"},
		{name: "HTML block", input: "<div>not an excerpt</div>\n\nfirst", expected: "first"},
		{name: "link reference", input: "[link]: /target\n\nfirst", expected: "first"},
		{name: "dollar math block", input: "$$\nx + y\n$$\nfirst", expected: "first"},
		{name: "square math block", input: "\\[\nx + y\n\\]\nfirst", expected: "first"},
		{name: "fenced code in blockquote", input: "> ```go\n> code\n> ```\n> first", expected: "> first"},
		{name: "fenced code in list", input: "- ```go\n  code\n  ```\n- first", expected: "- first"},
		{name: "suggestion", input: "```suggestion\nreplacement\n```\nexplanation", expected: "explanation"},
		{name: "tilde fence", input: "~~~go\ncode\n~~~\nfirst", expected: "first"},
		{name: "multiple code blocks", input: "```go\ncode\n```\n\n~~~rust\ncode\n~~~\nfirst", expected: "first"},
		{name: "indented code", input: "    code\n\nfirst", expected: "first"},
		{name: "code only", input: "```go\ncode\n```", expected: ""},
		{name: "unclosed fence", input: "```go\ncode", expected: ""},
		{name: "over-byte single line", input: strings.Repeat("x", excerptMaxBytes+1), expected: ""},
		{name: "prose beyond line limit", input: "```\n" + strings.Repeat("code\n", excerptMaxLines) + "```\nfirst", expected: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, FirstExcerptLine(test.input))
		})
	}
}

func TestFirstExcerptLineConcurrent(t *testing.T) {
	const goroutines = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			<-start
			for range 100 {
				assert.Equal(t, "first", FirstExcerptLine("```suggestion\nreplacement\n```\n\nfirst\nsecond"))
			}
		})
	}
	close(start)
	wg.Wait()
}
