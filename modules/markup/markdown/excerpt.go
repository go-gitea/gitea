// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"

	"gitea.dev/modules/markup/markdown/math"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var (
	excerptParser              = goldmarkExcerptParser()
	excerptAttentionRecognizer = NewASTTransformer(nil)
)

const (
	excerptMaxBytes = 64 * 1024
	excerptMaxLines = 256
)

func goldmarkExcerptParser() parser.Parser {
	p := goldmarkDefaultParser()
	p.AddOptions(math.BlockParserOption(true, true))
	return p
}

// FirstExcerptLine returns the first source line belonging to a Markdown paragraph, heading, or text block.
// It inspects at most 64 KiB and 256 complete physical lines.
func FirstExcerptLine(input string) string {
	source := excerptSource(input)
	reader := text.NewReader(source)
	document := excerptParser.Parse(reader)

	var excerpt []byte
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node.Kind() {
		case ast.KindParagraph, ast.KindHeading, ast.KindTextBlock:
			if node.ChildCount() == 0 {
				return ast.WalkSkipChildren, nil
			}

			pos := excerptNodePosition(node, reader)
			if pos < 0 {
				return ast.WalkSkipChildren, nil
			}
			lineStart := bytes.LastIndexByte(source[:pos], '\n') + 1
			lineEnd := len(source)
			if newline := bytes.IndexByte(source[pos:], '\n'); newline >= 0 {
				lineEnd = pos + newline
			}
			excerpt = bytes.TrimSpace(source[lineStart:lineEnd])
			if len(excerpt) == 0 {
				return ast.WalkSkipChildren, nil
			}
			return ast.WalkStop, nil
		case ast.KindDocument, ast.KindBlockquote, ast.KindList, ast.KindListItem:
			return ast.WalkContinue, nil
		default:
			return ast.WalkSkipChildren, nil
		}
	})

	return string(excerpt)
}

func excerptSource(input string) []byte {
	limit := min(len(input), excerptMaxBytes)
	lineCount, lastLineEnd := 0, 0
	for i := range limit {
		if input[i] != '\n' {
			continue
		}
		lineCount++
		lastLineEnd = i + 1
		if lineCount == excerptMaxLines {
			break
		}
	}
	if len(input) <= excerptMaxBytes && lineCount < excerptMaxLines {
		return []byte(input)
	}
	return []byte(input[:lastLineEnd])
}

func excerptNodePosition(node ast.Node, reader text.Reader) int {
	parent := node.Parent()
	if parent == nil || parent.Kind() != ast.KindBlockquote || parent.FirstChild() != node {
		return node.Pos()
	}

	_, marker := excerptAttentionRecognizer.extractBlockquoteAttention(node, reader)
	if len(marker) == 0 {
		return node.Pos()
	}
	for next := marker[len(marker)-1].NextSibling(); next != nil; next = next.NextSibling() {
		if textNode, ok := next.(*ast.Text); ok && len(textNode.Segment.Value(reader.Source())) == 0 {
			continue
		}
		return next.Pos()
	}
	return -1
}
