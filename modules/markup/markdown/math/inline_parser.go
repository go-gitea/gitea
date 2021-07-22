// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package math

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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

// Parse parses the current line and returns a result of parsing.
func (parser *inlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, startSegment := block.PeekLine()
	opener := bytes.Index(line, parser.start)
	if opener < 0 {
		return nil
	}
	opener += len(parser.start)
	block.Advance(opener)
	l, pos := block.Position()
	node := NewInline()

	for {
		line, segment := block.PeekLine()
		if line == nil {
			block.SetPosition(l, pos)
			return ast.NewTextSegment(startSegment.WithStop(startSegment.Start + opener))
		}

		closer := bytes.Index(line, parser.end)
		if closer < 0 {
			if !util.IsBlank(line) {
				node.AppendChild(node, ast.NewRawTextSegment(segment))
			}
			block.AdvanceLine()
			continue
		}
		segment = segment.WithStop(segment.Start + closer)
		if !segment.IsEmpty() {
			node.AppendChild(node, ast.NewRawTextSegment(segment))
		}
		block.Advance(closer + len(parser.end))
		break
	}

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
