// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"
	"strings"

	"gitea.dev/modules/markup/markdown/math"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const (
	excerptMaxBytes = 64 * 1024
	excerptMaxLines = 256
)

var excerptParser = goldmark.New(
	goldmark.WithParser(goldmarkDefaultParser()),
	goldmark.WithExtensions(math.NewExtension(nil, math.Options{Enabled: true, ParseBlockDollar: true, ParseBlockSquareBrackets: true})),
).Parser()

// FirstExcerptLine returns the first source line belonging to a Markdown paragraph, heading, or text block.
// It inspects at most 64 KiB and 256 complete physical lines.
func FirstExcerptLine(input string) string {
	reader := text.NewReader(excerptSource(input))
	return string(firstExcerptLine(excerptParser.Parse(reader), reader))
}

func firstExcerptLine(parent ast.Node, reader text.Reader) []byte {
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		var line []byte
		switch node.Kind() {
		case ast.KindBlockquote, ast.KindList, ast.KindListItem:
			line = firstExcerptLine(node, reader)
		case ast.KindParagraph, ast.KindHeading, ast.KindTextBlock:
			if pos := excerptNodePosition(node, reader); node.HasChildren() && pos >= 0 {
				source := reader.Source()
				line, _, _ = bytes.Cut(source[bytes.LastIndexByte(source[:pos], '\n')+1:], []byte{'\n'})
				line = bytes.TrimSpace(line)
			}
		}
		if len(line) > 0 {
			return line
		}
	}
	return nil
}

func excerptSource(input string) []byte {
	if len(input) > excerptMaxBytes {
		input = input[:strings.LastIndexByte(input[:excerptMaxBytes], '\n')+1]
	}
	end := 0
	for range excerptMaxLines {
		newline := strings.IndexByte(input[end:], '\n')
		if newline < 0 {
			return []byte(input)
		}
		end += newline + 1
	}
	return []byte(input[:end])
}

func excerptNodePosition(node ast.Node, reader text.Reader) int {
	parent := node.Parent()
	if parent == nil || parent.Kind() != ast.KindBlockquote || parent.FirstChild() != node {
		return node.Pos()
	}

	_, marker := extractBlockquoteAttention(node, reader)
	if len(marker) == 0 {
		return node.Pos()
	}
	for next := marker[len(marker)-1].NextSibling(); next != nil; next = next.NextSibling() {
		if t, ok := next.(*ast.Text); !ok || !t.Segment.IsEmpty() {
			return next.Pos()
		}
	}
	return -1
}
