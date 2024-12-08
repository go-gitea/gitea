// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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

var defaultDualDollarParser = &inlineParser{
	start: []byte{'$', '$'},
	end:   []byte{'$', '$'},
}

func NewInlineDollarParser() parser.InlineParser {
	return defaultInlineDollarParser
}

func NewInlineDualDollarParser() parser.InlineParser {
	return defaultDualDollarParser
}

var defaultInlineBracketParser = &inlineParser{
	start: []byte{'\\', '('},
	end:   []byte{'\\', ')'},
}

func NewInlineBracketParser() parser.InlineParser {
	return defaultInlineBracketParser
}

// Trigger triggers this parser on $ or \
func (parser *inlineParser) Trigger() []byte {
	return parser.start
}

func isPunctuation(b byte) bool {
	return b == '.' || b == '!' || b == '?' || b == ',' || b == ';' || b == ':'
}

func isBracket(b byte) bool {
	return b == ')'
}

func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// Parse parses the current line and returns a result of parsing.
func (parser *inlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()

	if !bytes.HasPrefix(line, parser.start) {
		// We'll catch this one on the next time round
		return nil
	}

	precedingCharacter := block.PrecendingCharacter()
	if precedingCharacter < 256 && (isAlphanumeric(byte(precedingCharacter)) || isPunctuation(byte(precedingCharacter))) {
		// need to exclude things like `a$` from being considered a start
		return nil
	}

	// move the opener marker point at the start of the text
	opener := len(parser.start)

	// Now look for an ending line
	depth := 0
	ender := -1
	for i := opener; i < len(line); i++ {
		if depth == 0 && bytes.HasPrefix(line[i:], parser.end) {
			succeedingCharacter := byte(0)
			if i+len(parser.end) < len(line) {
				succeedingCharacter = line[i+len(parser.end)]
			}
			// check valid ending character
			isValidEndingChar := isPunctuation(succeedingCharacter) || isBracket(succeedingCharacter) ||
				succeedingCharacter == ' ' || succeedingCharacter == '\n' || succeedingCharacter == 0
			if !isValidEndingChar {
				break
			}
			ender = i
			break
		}
		if line[i] == '\\' {
			i++
			continue
		}
		if line[i] == '{' {
			depth++
		} else if line[i] == '}' {
			depth--
		}
	}
	if ender == -1 {
		return nil
	}

	block.Advance(opener)
	_, pos := block.Position()
	var node ast.Node
	if parser == defaultDualDollarParser {
		node = NewInlineBlock()
	} else {
		node = NewInline()
	}
	segment := pos.WithStop(pos.Start + ender - opener)
	node.AppendChild(node, ast.NewRawTextSegment(segment))
	block.Advance(ender - opener + len(parser.end))

	if parser == defaultDualDollarParser {
		trimBlock(&(node.(*InlineBlock)).Inline, block)
	} else {
		trimBlock(node.(*Inline), block)
	}
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
