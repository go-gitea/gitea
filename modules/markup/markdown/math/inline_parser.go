// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package math

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type inlineParser struct {
	start []byte
	end   []byte
}

var defaultInlineDollarParser = &inlineParser{
	start: []byte{'$'},
	end:   []byte{'$'},
}

// NewInlineDollarParser returns a new inline parser
func NewInlineDollarParser() parser.InlineParser {
	return defaultInlineDollarParser
}

var defaultInlineBracketParser = &inlineParser{
	start: []byte{'\\', '('},
	end:   []byte{'\\', ')'},
}

// NewInlineDollarParser returns a new inline parser
func NewInlineBracketParser() parser.InlineParser {
	return defaultInlineBracketParser
}

// Trigger triggers this parser on $
func (parser *inlineParser) Trigger() []byte {
	return parser.start[0:1]
}

func isAlphanumeric(b byte) bool {
	// Github only cares about 0-9A-Za-z
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// Parse parses the current line and returns a result of parsing.
func (parser *inlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	opener := bytes.Index(line, parser.start)
	if opener < 0 {
		return nil
	}
	if opener != 0 && isAlphanumeric(line[opener-1]) {
		return nil
	}

	opener += len(parser.start)
	ender := bytes.Index(line[opener:], parser.end)
	if ender < 0 {
		return nil
	}
	if len(line) > opener+ender+len(parser.end) && isAlphanumeric(line[opener+ender+len(parser.end)]) {
		return nil
	}

	block.Advance(opener)
	_, pos := block.Position()
	node := NewInline()
	segment := pos.WithStop(pos.Start + ender)
	node.AppendChild(node, ast.NewRawTextSegment(segment))
	block.Advance(ender + len(parser.end))

	trimBlock(node, block)
	return node
}

func trimBlock(node *Inline, block text.Reader) {
	if node.IsBlank(block.Source()) {
		return
	}

	// trim first space and last space
	first := node.FirstChild().(*ast.Text)
	if !(!first.Segment.IsEmpty() && block.Source()[first.Segment.Start] == ' ') {
		return
	}

	last := node.LastChild().(*ast.Text)
	if !(!last.Segment.IsEmpty() && block.Source()[last.Segment.Stop-1] == ' ') {
		return
	}

	first.Segment = first.Segment.WithStart(first.Segment.Start + 1)
	last.Segment = last.Segment.WithStop(last.Segment.Stop - 1)
}
