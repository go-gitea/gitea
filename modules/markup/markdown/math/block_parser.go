// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package math

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type blockParser struct {
	parseDollars bool
}

type blockData struct {
	dollars bool
	indent  int
}

var blockInfoKey = parser.NewContextKey()

// NewBlockParser creates a new math BlockParser
func NewBlockParser(parseDollarBlocks bool) parser.BlockParser {
	return &blockParser{
		parseDollars: parseDollarBlocks,
	}
}

// Open parses the current line and returns a result of parsing.
func (b *blockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos == -1 || len(line[pos:]) < 2 {
		return nil, parser.NoChildren
	}

	dollars := false
	if b.parseDollars && line[pos] == '$' && line[pos+1] == '$' {
		dollars = true
	} else if line[pos] != '\\' || line[pos+1] != '[' {
		return nil, parser.NoChildren
	}

	pc.Set(blockInfoKey, &blockData{dollars: dollars, indent: pos})
	node := NewBlock()
	return node, parser.NoChildren
}

// Continue parses the current line and returns a result of parsing.
func (b *blockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, segment := reader.PeekLine()
	data := pc.Get(blockInfoKey).(*blockData)
	w, pos := util.IndentWidth(line, 0)
	if w < 4 {
		if data.dollars {
			i := pos
			for ; i < len(line) && line[i] == '$'; i++ {
			}
			length := i - pos
			if length >= 2 && util.IsBlank(line[i:]) {
				reader.Advance(segment.Stop - segment.Start - segment.Padding)
				return parser.Close
			}
		} else if len(line[pos:]) > 1 && line[pos] == '\\' && line[pos+1] == ']' && util.IsBlank(line[pos+2:]) {
			reader.Advance(segment.Stop - segment.Start - segment.Padding)
			return parser.Close
		}
	}

	pos, padding := util.IndentPosition(line, 0, data.indent)
	seg := text.NewSegmentPadding(segment.Start+pos, segment.Stop, padding)
	node.Lines().Append(seg)
	reader.AdvanceAndSetPadding(segment.Stop-segment.Start-pos-1, padding)
	return parser.Continue | parser.NoChildren
}

// Close will be called when the parser returns Close.
func (b *blockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	pc.Set(blockInfoKey, nil)
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
