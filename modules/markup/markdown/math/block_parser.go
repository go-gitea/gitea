// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package math

import (
	"bytes"

	giteaUtil "code.gitea.io/gitea/modules/util"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type blockParser struct {
	parseDollars    bool
	parseSquare     bool
	endBytesDollars []byte
	endBytesSquare  []byte
}

// NewBlockParser creates a new math BlockParser
func NewBlockParser(parseDollars, parseSquare bool) parser.BlockParser {
	return &blockParser{
		parseDollars:    parseDollars,
		parseSquare:     parseSquare,
		endBytesDollars: []byte{'$', '$'},
		endBytesSquare:  []byte{'\\', ']'},
	}
}

// Open parses the current line and returns a result of parsing.
func (b *blockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos == -1 || len(line[pos:]) < 2 {
		return nil, parser.NoChildren
	}

	var dollars bool
	if b.parseDollars && line[pos] == '$' && line[pos+1] == '$' {
		dollars = true
	} else if b.parseSquare && line[pos] == '\\' && line[pos+1] == '[' {
		if len(line[pos:]) >= 3 && line[pos+2] == '!' && bytes.Contains(line[pos:], []byte(`\]`)) {
			// do not process escaped attention block: "> \[!NOTE\]"
			return nil, parser.NoChildren
		}
		dollars = false
	} else {
		return nil, parser.NoChildren
	}

	node := NewBlock(dollars, pos)

	// Now we need to check if the ending block is on the segment...
	endBytes := giteaUtil.Iif(dollars, b.endBytesDollars, b.endBytesSquare)
	idx := bytes.Index(line[pos+2:], endBytes)
	if idx >= 0 {
		// for case: "$$ ... $$ any other text" (this case will be handled by the inline parser)
		for i := pos + 2 + idx + 2; i < len(line); i++ {
			if line[i] != ' ' && line[i] != '\n' {
				return nil, parser.NoChildren
			}
		}
		segment.Start += pos + 2
		segment.Stop = segment.Start + idx
		node.Lines().Append(segment)
		node.Closed = true
		node.Inline = true
		return node, parser.Close | parser.NoChildren
	}

	// for case "\[ ... ]" (no close marker on the same line)
	for i := pos + 2 + idx + 2; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\n' {
			return nil, parser.NoChildren
		}
	}

	segment.Start += pos + 2
	node.Lines().Append(segment)
	return node, parser.NoChildren
}

// Continue parses the current line and returns a result of parsing.
func (b *blockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	block := node.(*Block)
	if block.Closed {
		return parser.Close
	}

	line, segment := reader.PeekLine()
	w, pos := util.IndentWidth(line, reader.LineOffset())
	if w < 4 {
		endBytes := giteaUtil.Iif(block.Dollars, b.endBytesDollars, b.endBytesSquare)
		if bytes.HasPrefix(line[pos:], endBytes) && util.IsBlank(line[pos+len(endBytes):]) {
			if util.IsBlank(line[pos+len(endBytes):]) {
				newline := giteaUtil.Iif(line[len(line)-1] != '\n', 0, 1)
				reader.Advance(segment.Stop - segment.Start - newline + segment.Padding)
				return parser.Close
			}
		}
	}
	start := segment.Start + giteaUtil.Iif(pos > block.Indent, block.Indent, pos)
	seg := text.NewSegmentPadding(start, segment.Stop, segment.Padding)
	node.Lines().Append(seg)
	return parser.Continue | parser.NoChildren
}

// Close will be called when the parser returns Close.
func (b *blockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	// noop
}

// CanInterruptParagraph returns true if the parser can interrupt paragraphs,
// otherwise false.
func (b *blockParser) CanInterruptParagraph() bool {
	return true
}

// CanAcceptIndentedLine returns true if the parser can open new node when
// the given line is being indented more than 3 spaces.
func (b *blockParser) CanAcceptIndentedLine() bool {
	return false
}

// Trigger returns a list of characters that triggers Parse method of
// this parser.
// If Trigger returns a nil, Open will be called with any lines.
//
// We leave this as nil as our parse method is quick enough
func (b *blockParser) Trigger() []byte {
	return nil
}
