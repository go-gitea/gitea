/*  Original C version https://github.com/jgm/peg-markdown/
 *	Copyright 2008 John MacFarlane (jgm at berkeley dot edu).
 *
 *  Modifications and translation from C into Go
 *  based on markdown_lib.c and parsing_functions.c
 *	Copyright 2010 Michael Teichgräber (mt at wmipf dot de)
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License or the MIT
 *  license.  See LICENSE for details.
 *
 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 */

package rst

import (
	"bytes"
	"io"
	"log"
	"strings"
)

const (
	// If you get a build error message saying that
	// parserIfaceVersion_N is undefined, parser.leg.go
	// either is not present or it is out of date. You should
	// rebuild it using
	//	make nuke
	//	make parser
	needParserIfaceVersion = parserIfaceVersion_18
)

// reST Extensions.
type Extensions struct {
	Smart        bool
	Notes        bool
	FilterHTML   bool
	FilterStyles bool
	Strike       bool
	Dlists       bool
}

type Parser struct {
	yy           yyParser
	preformatBuf *bytes.Buffer
}

// NewParser creates an instance of a parser. It can be reused
// so that stacks and buffers need not be allocated anew for
// each reST call.
func NewParser(x *Extensions) (p *Parser) {
	p = new(Parser)
	if x != nil {
		p.yy.state.extension = *x
	}
	p.yy.Init()
	p.yy.state.heap.init(1024)
	p.preformatBuf = bytes.NewBuffer(make([]byte, 0, 32768))
	initParser()
	return
}

// A Formatter is called repeatedly, one reST block at a time,
// while the document is parsed. At the end of a document the Finish
// method is called, which may, for example, print footnotes.
// A Formatter can be reused.
type Formatter interface {
	FormatBlock(*element)
	Finish()
}

// reST parses input from an io.Reader into a tree, and sends
// parsed blocks to a Formatter
func (p *Parser) ReStructuredText(src io.Reader, f Formatter) {
	s := p.preformat(src)

	p.parseRule(ruleReferences, s)
	if p.yy.extension.Notes {
		p.parseRule(ruleNotes, s)
	}
	p.yy.state.heap.Reset()

	for {
		tree := p.parseRule(ruleDocblock, s)
		if tree == nil {
			break
		}
		s = p.yy.ResetBuffer("")
		tree = p.processRawBlocks(tree)
		f.FormatBlock(tree)

		p.yy.state.heap.Reset()
	}
	f.Finish()
}

func (p *Parser) parseRule(rule int, s string) (tree *element) {
	old := p.yy.ResetBuffer(s)
	if old != "" && strings.Trim(old, "\r\n ") != "" {
		log.Fatalln("Buffer not empty", "["+old+"]")
	}
	err := p.yy.Parse(rule)
	switch rule {
	case ruleDoc, ruleDocblock:
		if err == nil {
			tree = p.yy.state.tree
		}
		p.yy.state.tree = nil
	}
	return
}

/* process_raw_blocks - traverses an element list, replacing any RAW elements with
 * the result of parsing them as markdown text, and recursing into the children
 * of parent elements.  The result should be a tree of elements without any RAWs.
 */
func (p *Parser) processRawBlocks(input *element) *element {

	for current := input; current != nil; current = current.next {
		if current.key == RAW {
			/* \001 is used to indicate boundaries between nested lists when there
			 * is no blank line.  We split the string by \001 and parse
			 * each chunk separately.
			 */
			current.key = LIST
			current.children = nil
			listEnd := &current.children
			for _, contents := range strings.Split(current.contents.str, "\001") {
				if list := p.parseRule(ruleDoc, contents); list != nil {
					*listEnd = list
					for list.next != nil {
						list = list.next
					}
					listEnd = &list.next
				}
			}
			current.contents.str = ""
		}
		if current.children != nil {
			current.children = p.processRawBlocks(current.children)
		}
	}
	return input
}

const (
	TABSTOP = 4
)

/* preformat - allocate and copy text buffer while
 * performing tab expansion.
 */
func (p *Parser) preformat(r io.Reader) (s string) {
	charstotab := TABSTOP
	buf := make([]byte, 32768)

	b := p.preformatBuf
	b.Reset()
	for {
		n, err := r.Read(buf)
		if err != nil {
			break
		}
		i0 := 0
		for i, c := range buf[:n] {
			switch c {
			case '\t':
				b.Write(buf[i0:i])
				for ; charstotab > 0; charstotab-- {
					b.WriteByte(' ')
				}
				i0 = i + 1
			case '\n':
				b.Write(buf[i0 : i+1])
				i0 = i + 1
				charstotab = TABSTOP
			default:
				charstotab--
			}
			if charstotab == 0 {
				charstotab = TABSTOP
			}
		}
		b.Write(buf[i0:n])
	}

	b.WriteString("\n\n")
	return b.String()
}
