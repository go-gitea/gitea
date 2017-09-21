/*  Original C version https://github.com/jgm/peg-markdown/
 *	Copyright 2008 John MacFarlane (jgm at berkeley dot edu).
 *
 *  Modifications and translation from C into Go
 *  based on markdown_parser.leg and utility_functions.c
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

// PEG grammar and parser actions for restructuredtext syntax.

import (
	"fmt"
	"io"
	"log"
	"strings"
)

const (
	parserIfaceVersion_18 = iota
)

// Semantic value of a parsing action.
type element struct {
	key int
	contents
	children *element
	next     *element
}

// Information (label, URL and title) for a link.
type link struct {
	label *element
	url   string
	title string
}

// code contents
type code struct {
	lang string
}

// Union for contents of an Element (string, list, code, or link).
type contents struct {
	str string
	*link
	*code
}

// Types of semantic values returned by parsers.
const (
	LIST = iota /* A generic list of values. For ordered and bullet lists, see below. */
	RAW         /* Raw markdown to be processed further */
	SPACE
	LINEBREAK
	ELLIPSIS
	EMDASH
	ENDASH
	APOSTROPHE
	SINGLEQUOTED
	DOUBLEQUOTED
	STR
	LINK
	IMAGE
	CODE
	HTML
	EMPH
	STRONG
	STRIKE
	PLAIN
	PARA
	LISTITEM
	BULLETLIST
	ORDEREDLIST
	H1TITLE
	H1 /* Code assumes that H1..6 are in order. */
	H2
	H3
	H4
	H5
	H6
	TABLE
	TABLESEPARATOR
	TABLECELL
	CELLSPAN
	TABLEROW
	TABLEBODY
	TABLEHEAD
	CODEBLOCK
	BLOCKQUOTE
	VERBATIM
	HTMLBLOCK
	HRULE
	REFERENCE
	NOTE
	DEFINITIONLIST
	DEFTITLE
	DEFDATA
	numVAL
)

type state struct {
	extension  Extensions
	heap       elemHeap
	tree       *element /* Results of parse. */
	references *element /* List of link references found. */
	notes      *element /* List of footnotes found. */
}

const (
	ruleDoc = iota
	ruleDocblock
	ruleBlock
	rulePara
	rulePlain
	ruleSetextBottom
	ruleHeadingTitle
	ruleHeading
	ruleImage
	ruleCodeBlock
	ruleDoctestBlock
	ruleBlockQuoteRaw
	ruleBlockQuoteChunk
	ruleBlockQuote
	ruleNonblankIndentedLine
	ruleVerbatimChunk
	ruleVerbatim
	ruleHorizontalRule
	ruleTable
	ruleSimpleTable
	ruleGridTable
	ruleHeaderLessGridTable
	ruleGridTableHeader
	ruleGridTableBody
	ruleGridTableRow
	ruleTableCell
	ruleGridTableHeaderSep
	ruleGridTableSep
	ruleBullet
	ruleBulletList
	ruleListTight
	ruleListLoose
	ruleListItem
	ruleListItemTight
	ruleListBlock
	ruleListContinuationBlock
	ruleEnumerator
	ruleOrderedList
	ruleListBlockLine
	ruleHtmlBlockOpenAddress
	ruleHtmlBlockCloseAddress
	ruleHtmlBlockAddress
	ruleHtmlBlockOpenBlockquote
	ruleHtmlBlockCloseBlockquote
	ruleHtmlBlockBlockquote
	ruleHtmlBlockOpenCenter
	ruleHtmlBlockCloseCenter
	ruleHtmlBlockCenter
	ruleHtmlBlockOpenDir
	ruleHtmlBlockCloseDir
	ruleHtmlBlockDir
	ruleHtmlBlockOpenDiv
	ruleHtmlBlockCloseDiv
	ruleHtmlBlockDiv
	ruleHtmlBlockOpenDl
	ruleHtmlBlockCloseDl
	ruleHtmlBlockDl
	ruleHtmlBlockOpenFieldset
	ruleHtmlBlockCloseFieldset
	ruleHtmlBlockFieldset
	ruleHtmlBlockOpenForm
	ruleHtmlBlockCloseForm
	ruleHtmlBlockForm
	ruleHtmlBlockOpenH1
	ruleHtmlBlockCloseH1
	ruleHtmlBlockH1
	ruleHtmlBlockOpenH2
	ruleHtmlBlockCloseH2
	ruleHtmlBlockH2
	ruleHtmlBlockOpenH3
	ruleHtmlBlockCloseH3
	ruleHtmlBlockH3
	ruleHtmlBlockOpenH4
	ruleHtmlBlockCloseH4
	ruleHtmlBlockH4
	ruleHtmlBlockOpenH5
	ruleHtmlBlockCloseH5
	ruleHtmlBlockH5
	ruleHtmlBlockOpenH6
	ruleHtmlBlockCloseH6
	ruleHtmlBlockH6
	ruleHtmlBlockOpenMenu
	ruleHtmlBlockCloseMenu
	ruleHtmlBlockMenu
	ruleHtmlBlockOpenNoframes
	ruleHtmlBlockCloseNoframes
	ruleHtmlBlockNoframes
	ruleHtmlBlockOpenNoscript
	ruleHtmlBlockCloseNoscript
	ruleHtmlBlockNoscript
	ruleHtmlBlockOpenOl
	ruleHtmlBlockCloseOl
	ruleHtmlBlockOl
	ruleHtmlBlockOpenP
	ruleHtmlBlockCloseP
	ruleHtmlBlockP
	ruleHtmlBlockOpenPre
	ruleHtmlBlockClosePre
	ruleHtmlBlockPre
	ruleHtmlBlockOpenTable
	ruleHtmlBlockCloseTable
	ruleHtmlBlockTable
	ruleHtmlBlockOpenUl
	ruleHtmlBlockCloseUl
	ruleHtmlBlockUl
	ruleHtmlBlockOpenDd
	ruleHtmlBlockCloseDd
	ruleHtmlBlockDd
	ruleHtmlBlockOpenDt
	ruleHtmlBlockCloseDt
	ruleHtmlBlockDt
	ruleHtmlBlockOpenFrameset
	ruleHtmlBlockCloseFrameset
	ruleHtmlBlockFrameset
	ruleHtmlBlockOpenLi
	ruleHtmlBlockCloseLi
	ruleHtmlBlockLi
	ruleHtmlBlockOpenTbody
	ruleHtmlBlockCloseTbody
	ruleHtmlBlockTbody
	ruleHtmlBlockOpenTd
	ruleHtmlBlockCloseTd
	ruleHtmlBlockTd
	ruleHtmlBlockOpenTfoot
	ruleHtmlBlockCloseTfoot
	ruleHtmlBlockTfoot
	ruleHtmlBlockOpenTh
	ruleHtmlBlockCloseTh
	ruleHtmlBlockTh
	ruleHtmlBlockOpenThead
	ruleHtmlBlockCloseThead
	ruleHtmlBlockThead
	ruleHtmlBlockOpenTr
	ruleHtmlBlockCloseTr
	ruleHtmlBlockTr
	ruleHtmlBlockOpenScript
	ruleHtmlBlockCloseScript
	ruleHtmlBlockScript
	ruleHtmlBlockOpenHead
	ruleHtmlBlockCloseHead
	ruleHtmlBlockHead
	ruleHtmlBlockInTags
	ruleHtmlBlock
	ruleHtmlBlockSelfClosing
	ruleHtmlBlockType
	ruleStyleOpen
	ruleStyleClose
	ruleInStyleTags
	ruleStyleBlock
	ruleInlines
	ruleInline
	ruleSpace
	ruleStr
	ruleStrChunk
	ruleAposChunk
	ruleEscapedChar
	ruleEntity
	ruleEndline
	ruleNormalEndline
	ruleTerminalEndline
	ruleLineBreak
	ruleSymbol
	ruleApplicationDepent
	ruleUlOrStarLine
	ruleStarLine
	ruleUlLine
	ruleWhitespace
	ruleEmph
	ruleStrong
	ruleStrike
	ruleLink
	ruleReferenceLink
	ruleUnquotedRefLinkUnderbar
	ruleQuotedRefLinkUnderbar
	ruleExplicitLink
	ruleSource
	ruleSourceContents
	ruleTitle
	ruleTitleSingle
	ruleTitleDouble
	ruleAutoLink
	ruleEmbeddedLink
	ruleAutoLinkUrl
	ruleAutoLinkEmail
	ruleReference
	ruleQuotedReference
	ruleUnquotedReference
	ruleUrlReference
	ruleUnquotedLinkSource
	ruleRefSource
	ruleQuotedRefSource
	ruleEmbeddedRefSource
	ruleLabel
	ruleRefSrc
	ruleReferences
	ruleTicks2
	ruleCode
	ruleRawHtml
	ruleBlankLine
	ruleQuoted
	ruleHtmlAttribute
	ruleHtmlComment
	ruleHtmlTag
	ruleEof
	ruleSpacechar
	ruleNonspacechar
	ruleNewline
	ruleSp
	ruleSpnl
	ruleSpecialChar
	ruleNormalChar
	ruleAlphanumeric
	ruleAlphanumericAscii
	ruleDigit
	ruleHexEntity
	ruleDecEntity
	ruleCharEntity
	ruleNonindentSpace
	ruleIndent
	ruleIndentedLine
	ruleOptionallyIndentedLine
	ruleStartList
	ruleDoctestLine
	ruleLine
	ruleRawLine
	ruleSkipBlock
	ruleExtendedSpecialChar
	ruleSmart
	ruleApostrophe
	ruleEllipsis
	ruleDash
	ruleEnDash
	ruleEmDash
	ruleSingleQuoteStart
	ruleSingleQuoteEnd
	ruleSingleQuoted
	ruleDoubleQuoteStart
	ruleDoubleQuoteEnd
	ruleDoubleQuoted
	ruleNoteReference
	ruleRawNoteReference
	ruleNote
	ruleFootnote
	ruleNotes
	ruleRawNoteBlock
	ruleDefinitionList
	ruleDefinition
	ruleDListTitle
	ruleDefTight
	ruleDefLoose
	ruleDefmark
	ruleDefMarker
)

type yyParser struct {
	state
	Buffer      string
	Min, Max    int
	rules       [252]func() bool
	commit      func(int) bool
	ResetBuffer func(string) string
}

func (p *yyParser) Parse(ruleId int) (err error) {
	if p.rules[ruleId]() {
		// Make sure thunkPosition is 0 (there may be a yyPop action on the stack).
		p.commit(0)
		return
	}
	return p.parseErr()
}

type errPos struct {
	Line, Pos int
}

func (e *errPos) String() string {
	return fmt.Sprintf("%d:%d", e.Line, e.Pos)
}

type unexpectedCharError struct {
	After, At errPos
	Char      byte
}

func (e *unexpectedCharError) Error() string {
	return fmt.Sprintf("%v: unexpected character '%c'", &e.At, e.Char)
}

type unexpectedEOFError struct {
	After errPos
}

func (e *unexpectedEOFError) Error() string {
	return fmt.Sprintf("%v: unexpected end of file", &e.After)
}

func (p *yyParser) parseErr() (err error) {
	var pos, after errPos
	pos.Line = 1
	for i, c := range p.Buffer[0:] {
		if c == '\n' {
			pos.Line++
			pos.Pos = 0
		} else {
			pos.Pos++
		}
		if i == p.Min {
			if p.Min != p.Max {
				after = pos
			} else {
				break
			}
		} else if i == p.Max {
			break
		}
	}
	if p.Max >= len(p.Buffer) {
		if p.Min == p.Max {
			err = io.EOF
		} else {
			err = &unexpectedEOFError{after}
		}
	} else {
		err = &unexpectedCharError{after, pos, p.Buffer[p.Max]}
	}
	return
}

func (p *yyParser) Init() {
	var position int
	var yyp int
	var yy *element
	var yyval = make([]*element, 256)

	actions := [...]func(string, int){
		/* 0 Doc */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 1 Doc */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			p.tree = reverse(a)
			yyval[yyp-1] = a
		},
		/* 2 Docblock */
		func(yytext string, _ int) {
			p.tree = yy
		},
		/* 3 Para */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = a
			yy.key = PARA
			yyval[yyp-1] = a
		},
		/* 4 Plain */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = a
			yy.key = PLAIN
			yyval[yyp-1] = a
		},
		/* 5 HeadingTitle */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 6 HeadingTitle */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkList(H1TITLE, a)
			yyval[yyp-1] = a
		},
		/* 7 Heading */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 8 Heading */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkList(getHeadingElm(string(yytext[0])), a)
			yyval[yyp-1] = a
		},
		/* 9 Image */
		func(yytext string, _ int) {
			l := yyval[yyp-1]
			a := yyval[yyp-2]
			t := yyval[yyp-3]
			g := yyval[yyp-4]

			tt := p.mkElem(LIST)
			title := ""
			if a != nil && a.contents.str != "" {
				title = a.contents.str
			}
			tt = p.mkLink(p.mkString(l.contents.str), l.contents.str, title)
			tt.key = IMAGE

			if t != nil {
				gg := p.mkLink(p.mkString(t.contents.str), t.contents.str, "")
				gg.children = tt
				yy = gg
			} else {
				yy = tt
				yy.children = nil
			}
			a = nil
			t = nil
			g = nil
			l = nil

			yyval[yyp-1] = l
			yyval[yyp-2] = a
			yyval[yyp-3] = t
			yyval[yyp-4] = g
		},
		/* 10 CodeBlock */
		func(yytext string, _ int) {
			l := yyval[yyp-1]
			a := yyval[yyp-2]
			a = cons(yy, a)
			yyval[yyp-1] = l
			yyval[yyp-2] = a
		},
		/* 11 CodeBlock */
		func(yytext string, _ int) {
			l := yyval[yyp-1]
			a := yyval[yyp-2]

			yy = p.mkCodeBlock(a, l.contents.str)
			l = nil

			yyval[yyp-1] = l
			yyval[yyp-2] = a
		},
		/* 12 DoctestBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 13 DoctestBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 14 DoctestBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			yy = p.mkCodeBlock(a, "python")

			yyval[yyp-1] = a
		},
		/* 15 BlockQuoteRaw */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 16 BlockQuoteRaw */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			yy = p.mkStringFromList(a, true)
			yy.key = RAW

			yyval[yyp-1] = a
		},
		/* 17 BlockQuoteChunk */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(p.mkString("\n"), a)
			yyval[yyp-1] = a
		},
		/* 18 BlockQuoteChunk */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 19 BlockQuoteChunk */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkStringFromList(a, false)
			yyval[yyp-1] = a
		},
		/* 20 BlockQuote */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 21 BlockQuote */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkElem(BLOCKQUOTE)
			yy.children = a
			yyval[yyp-1] = a
		},
		/* 22 VerbatimChunk */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(p.mkString("\n"), a)
			yyval[yyp-1] = a
		},
		/* 23 VerbatimChunk */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 24 VerbatimChunk */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkStringFromList(a, false)
			yyval[yyp-1] = a
		},
		/* 25 Verbatim */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 26 Verbatim */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkStringFromList(a, false)
			yy.key = VERBATIM
			yyval[yyp-1] = a
		},
		/* 27 HorizontalRule */
		func(yytext string, _ int) {
			yy = p.mkElem(HRULE)
		},
		/* 28 GridTable */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy.key = TABLEHEAD
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 29 GridTable */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = yy
			yyval[yyp-1] = a
		},
		/* 30 GridTable */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 31 GridTable */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			yy = p.mkList(TABLE, a)

			yyval[yyp-1] = a
		},
		/* 32 HeaderLessGridTable */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 33 HeaderLessGridTable */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			yy = p.mkList(TABLE, a)

			yyval[yyp-1] = a
		},
		/* 34 GridTableHeader */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 35 GridTableHeader */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			yy = p.mkList(TABLEBODY, a)

			yyval[yyp-1] = a
		},
		/* 36 GridTableBody */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 37 GridTableBody */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkList(TABLEBODY, a)
			yyval[yyp-1] = a
		},
		/* 38 GridTableRow */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			// TODO: support inline text
			raw := p.mkString(yytext)
			raw.key = RAW
			yy = p.mkElem(TABLECELL)
			yy.children = raw
			a = cons(yy, a)

			yyval[yyp-1] = a
		},
		/* 39 GridTableRow */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			yy = p.mkList(TABLEROW, a)

			yyval[yyp-1] = a
		},
		/* 40 TableCell */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			yyval[yyp-1] = a
		},
		/* 41 BulletList */
		func(yytext string, _ int) {
			yy.key = BULLETLIST
		},
		/* 42 ListTight */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 43 ListTight */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkList(LIST, a)
			yyval[yyp-1] = a
		},
		/* 44 ListLoose */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]

			li := b.children
			li.contents.str += "\n\n"
			a = cons(b, a)

			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 45 ListLoose */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			yy = p.mkList(LIST, a)
			yyval[yyp-2] = b
			yyval[yyp-1] = a
		},
		/* 46 ListItem */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 47 ListItem */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 48 ListItem */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			raw := p.mkStringFromList(a, false)
			raw.key = RAW
			yy = p.mkElem(LISTITEM)
			yy.children = raw

			yyval[yyp-1] = a
		},
		/* 49 ListItemTight */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 50 ListItemTight */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 51 ListItemTight */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			raw := p.mkStringFromList(a, false)
			raw.key = RAW
			yy = p.mkElem(LISTITEM)
			yy.children = raw

			yyval[yyp-1] = a
		},
		/* 52 ListBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 53 ListBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 54 ListBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkStringFromList(a, false)
			yyval[yyp-1] = a
		},
		/* 55 ListContinuationBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			if len(yytext) == 0 {
				a = cons(p.mkString("\001"), a) // block separator
			} else {
				a = cons(p.mkString(yytext), a)
			}

			yyval[yyp-1] = a
		},
		/* 56 ListContinuationBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 57 ListContinuationBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkStringFromList(a, false)
			yyval[yyp-1] = a
		},
		/* 58 OrderedList */
		func(yytext string, _ int) {
			yy.key = ORDEREDLIST
		},
		/* 59 HtmlBlock */
		func(yytext string, _ int) {
			if p.extension.FilterHTML {
				yy = p.mkList(LIST, nil)
			} else {
				yy = p.mkString(yytext)
				yy.key = HTMLBLOCK
			}

		},
		/* 60 StyleBlock */
		func(yytext string, _ int) {
			if p.extension.FilterStyles {
				yy = p.mkList(LIST, nil)
			} else {
				yy = p.mkString(yytext)
				yy.key = HTMLBLOCK
			}

		},
		/* 61 Inlines */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			c := yyval[yyp-2]
			a = cons(yy, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = c
		},
		/* 62 Inlines */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			c := yyval[yyp-2]
			a = cons(c, a)
			yyval[yyp-2] = c
			yyval[yyp-1] = a
		},
		/* 63 Inlines */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			c := yyval[yyp-2]

			yy = p.mkList(LIST, a)

			yyval[yyp-1] = a
			yyval[yyp-2] = c
		},
		/* 64 Space */
		func(yytext string, _ int) {
			yy = p.mkString(" ")
			yy.key = SPACE
		},
		/* 65 Str */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(p.mkString(yytext), a)
			yyval[yyp-1] = a
		},
		/* 66 Str */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 67 Str */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			if a.next == nil {
				yy = a
			} else {
				yy = p.mkList(LIST, a)
			}
			yyval[yyp-1] = a
		},
		/* 68 StrChunk */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 69 AposChunk */
		func(yytext string, _ int) {
			yy = p.mkElem(APOSTROPHE)
		},
		/* 70 EscapedChar */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 71 Entity */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
			yy.key = HTML
		},
		/* 72 NormalEndline */
		func(yytext string, _ int) {
			yy = p.mkString("\n")
			yy.key = SPACE
		},
		/* 73 TerminalEndline */
		func(yytext string, _ int) {
			yy = nil
		},
		/* 74 LineBreak */
		func(yytext string, _ int) {
			yy = p.mkElem(LINEBREAK)
		},
		/* 75 Symbol */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 76 ApplicationDepent */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkString(yytext)
			a = nil
			yyval[yyp-1] = a
		},
		/* 77 UlOrStarLine */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 78 Emph */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			a = cons(b, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 79 Emph */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			yy = p.mkList(EMPH, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 80 Strong */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			a = cons(b, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 81 Strong */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			yy = p.mkList(STRONG, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 82 Strike */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			a = cons(b, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 83 Strike */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			yy = p.mkList(STRIKE, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 84 UnquotedRefLinkUnderbar */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			if match, found := p.findReference(p.mkString(a.contents.str)); found {
				yy = p.mkLink(p.mkString(a.contents.str), match.url, match.title)
				a = nil
			} else {
				result := p.mkElem(LIST)
				result.children = cons(p.mkString(a.contents.str), cons(a, p.mkString(yytext)))
				yy = result
			}

			yyval[yyp-1] = a
		},
		/* 85 QuotedRefLinkUnderbar */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			if match, found := p.findReference(p.mkString(a.contents.str)); found {
				yy = p.mkLink(p.mkString(yytext), match.url, match.title)
				a = nil
			} else {
				result := p.mkElem(LIST)
				result.children = cons(p.mkString(a.contents.str), cons(a, p.mkString(yytext)))
				yy = result
			}

			yyval[yyp-1] = a
		},
		/* 86 ExplicitLink */
		func(yytext string, _ int) {
			t := yyval[yyp-1]
			l := yyval[yyp-2]
			s := yyval[yyp-3]

			yy = p.mkLink(l.children, s.contents.str, t.contents.str)
			s = nil
			t = nil
			l = nil

			yyval[yyp-2] = l
			yyval[yyp-3] = s
			yyval[yyp-1] = t
		},
		/* 87 Source */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 88 Title */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 89 EmbeddedLink */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			l := yyval[yyp-2]

			yy = p.mkLink(p.mkString(l.contents.str), yytext, "")

			yyval[yyp-1] = a
			yyval[yyp-2] = l
		},
		/* 90 AutoLinkUrl */
		func(yytext string, _ int) {
			yy = p.mkLink(p.mkString(yytext), yytext, "")
		},
		/* 91 AutoLinkEmail */
		func(yytext string, _ int) {

			yy = p.mkLink(p.mkString(yytext), "mailto:"+yytext, "")

		},
		/* 92 QuotedReference */
		func(yytext string, _ int) {
			c := yyval[yyp-1]
			s := yyval[yyp-2]

			yy = p.mkLink(p.mkString(c.contents.str), s.contents.str, "")
			s = nil
			c = nil
			yy.key = REFERENCE

			yyval[yyp-1] = c
			yyval[yyp-2] = s
		},
		/* 93 UnquotedReference */
		func(yytext string, _ int) {
			c := yyval[yyp-1]
			s := yyval[yyp-2]

			yy = p.mkLink(p.mkString(c.contents.str), s.contents.str, "")
			s = nil
			c = nil
			yy.key = REFERENCE

			yyval[yyp-1] = c
			yyval[yyp-2] = s
		},
		/* 94 UrlReference */
		func(yytext string, _ int) {
			s := yyval[yyp-1]

			yy = p.mkLink(p.mkString(yytext), yytext, "")
			s = nil
			yy.key = REFERENCE

			yyval[yyp-1] = s
		},
		/* 95 UnquotedLinkSource */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 96 RefSource */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 97 QuotedRefSource */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 98 EmbeddedRefSource */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 99 Label */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 100 Label */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			yy = p.mkList(LIST, a)

			yyval[yyp-1] = a
		},
		/* 101 RefSrc */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
			yy.key = HTML
		},
		/* 102 References */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			a = cons(b, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 103 References */
		func(yytext string, _ int) {
			b := yyval[yyp-2]
			a := yyval[yyp-1]
			p.references = reverse(a)
			p.state.heap.hasGlobals = true

			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 104 Code */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
			yy.key = CODE
		},
		/* 105 RawHtml */
		func(yytext string, _ int) {
			if p.extension.FilterHTML {
				yy = p.mkList(LIST, nil)
			} else {
				yy = p.mkString(yytext)
				yy.key = HTML
			}

		},
		/* 106 StartList */
		func(yytext string, _ int) {
			yy = nil
		},
		/* 107 DoctestLine */
		func(yytext string, _ int) {
			yy = p.mkString(">>> " + yytext)
		},
		/* 108 Line */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 109 Apostrophe */
		func(yytext string, _ int) {
			yy = p.mkElem(APOSTROPHE)
		},
		/* 110 Ellipsis */
		func(yytext string, _ int) {
			yy = p.mkElem(ELLIPSIS)
		},
		/* 111 EnDash */
		func(yytext string, _ int) {
			yy = p.mkElem(ENDASH)
		},
		/* 112 EmDash */
		func(yytext string, _ int) {
			yy = p.mkElem(EMDASH)
		},
		/* 113 SingleQuoted */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			a = cons(b, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 114 SingleQuoted */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			yy = p.mkList(SINGLEQUOTED, a)
			yyval[yyp-2] = b
			yyval[yyp-1] = a
		},
		/* 115 DoubleQuoted */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			a = cons(b, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 116 DoubleQuoted */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			yy = p.mkList(DOUBLEQUOTED, a)
			yyval[yyp-2] = b
			yyval[yyp-1] = a
		},
		/* 117 NoteReference */
		func(yytext string, _ int) {
			ref := yyval[yyp-1]

			p.state.heap.hasGlobals = true
			if match, ok := p.find_note(ref.contents.str); ok {
				yy = p.mkElem(NOTE)
				yy.children = match.children
				yy.contents.str = ""
			} else {
				yy = p.mkString("[^" + ref.contents.str + "]")
			}

			yyval[yyp-1] = ref
		},
		/* 118 RawNoteReference */
		func(yytext string, _ int) {
			yy = p.mkString(yytext)
		},
		/* 119 Note */
		func(yytext string, _ int) {
			ref := yyval[yyp-1]
			a := yyval[yyp-2]
			a = cons(yy, a)
			yyval[yyp-1] = ref
			yyval[yyp-2] = a
		},
		/* 120 Note */
		func(yytext string, _ int) {
			ref := yyval[yyp-1]
			a := yyval[yyp-2]
			a = cons(yy, a)
			yyval[yyp-1] = ref
			yyval[yyp-2] = a
		},
		/* 121 Note */
		func(yytext string, _ int) {
			ref := yyval[yyp-1]
			a := yyval[yyp-2]
			yy = p.mkList(NOTE, a)
			yy.contents.str = ref.contents.str

			yyval[yyp-1] = ref
			yyval[yyp-2] = a
		},
		/* 122 Footnote */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 123 Footnote */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			yy = p.mkList(NOTE, a)
			//p.state.heap.hasGlobals = true
			yy.contents.str = ""

			yyval[yyp-1] = a
		},
		/* 124 Notes */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			a = cons(b, a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 125 Notes */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			b := yyval[yyp-2]
			p.notes = reverse(a)
			yyval[yyp-1] = a
			yyval[yyp-2] = b
		},
		/* 126 RawNoteBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 127 RawNoteBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(p.mkString(yytext), a)
			yyval[yyp-1] = a
		},
		/* 128 RawNoteBlock */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkStringFromList(a, true)
			p.state.heap.hasGlobals = true
			yy.key = RAW

			yyval[yyp-1] = a
		},
		/* 129 DefinitionList */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 130 DefinitionList */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkList(DEFINITIONLIST, a)
			yyval[yyp-1] = a
		},
		/* 131 Definition */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 132 Definition */
		func(yytext string, _ int) {
			a := yyval[yyp-1]

			for e := yy.children; e != nil; e = e.next {
				e.key = DEFDATA
			}
			a = cons(yy, a)

			yyval[yyp-1] = a
		},
		/* 133 Definition */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkList(LIST, a)
			yyval[yyp-1] = a
		},
		/* 134 DListTitle */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			a = cons(yy, a)
			yyval[yyp-1] = a
		},
		/* 135 DListTitle */
		func(yytext string, _ int) {
			a := yyval[yyp-1]
			yy = p.mkList(LIST, a)
			yy.key = DEFTITLE

			yyval[yyp-1] = a
		},

		/* yyPush */
		func(_ string, count int) {
			yyp += count
			if yyp >= len(yyval) {
				s := make([]*element, cap(yyval)+256)
				copy(s, yyval)
				yyval = s
			}
		},
		/* yyPop */
		func(_ string, count int) {
			yyp -= count
		},
		/* yySet */
		func(_ string, count int) {
			yyval[yyp+count] = yy
		},
	}
	const (
		yyPush = 136 + iota
		yyPop
		yySet
	)

	type thunk struct {
		action     uint16
		begin, end int
	}
	var thunkPosition, begin, end int
	thunks := make([]thunk, 32)
	doarg := func(action uint16, arg int) {
		if thunkPosition == len(thunks) {
			newThunks := make([]thunk, 2*len(thunks))
			copy(newThunks, thunks)
			thunks = newThunks
		}
		t := &thunks[thunkPosition]
		thunkPosition++
		t.action = action
		if arg != 0 {
			t.begin = arg // use begin to store an argument
		} else {
			t.begin = begin
		}
		t.end = end
	}
	do := func(action uint16) {
		doarg(action, 0)
	}

	p.ResetBuffer = func(s string) (old string) {
		if position < len(p.Buffer) {
			old = p.Buffer[position:]
		}
		p.Buffer = s
		thunkPosition = 0
		position = 0
		p.Min = 0
		p.Max = 0
		end = 0
		return
	}

	p.commit = func(thunkPosition0 int) bool {
		if thunkPosition0 == 0 {
			s := ""
			for _, t := range thunks[:thunkPosition] {
				b := t.begin
				if b >= 0 && b <= t.end {
					s = p.Buffer[b:t.end]
				}
				magic := b
				actions[t.action](s, magic)
			}
			p.Min = position
			thunkPosition = 0
			return true
		}
		return false
	}
	matchDot := func() bool {
		if position < len(p.Buffer) {
			position++
			return true
		} else if position >= p.Max {
			p.Max = position
		}
		return false
	}

	matchChar := func(c byte) bool {
		if (position < len(p.Buffer)) && (p.Buffer[position] == c) {
			position++
			return true
		} else if position >= p.Max {
			p.Max = position
		}
		return false
	}

	peekChar := func(c byte) bool {
		return position < len(p.Buffer) && p.Buffer[position] == c
	}

	matchString := func(s string) bool {
		length := len(s)
		next := position + length
		if (next <= len(p.Buffer)) && p.Buffer[position] == s[0] && (p.Buffer[position:next] == s) {
			position = next
			return true
		} else if position >= p.Max {
			p.Max = position
		}
		return false
	}

	classes := [...][32]uint8{
		3: {0, 0, 0, 0, 50, 232, 255, 3, 254, 255, 255, 135, 254, 255, 255, 71, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		1: {0, 0, 0, 0, 10, 111, 0, 80, 0, 0, 0, 184, 1, 0, 0, 56, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		0: {0, 0, 0, 0, 0, 0, 255, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		4: {0, 0, 0, 0, 0, 0, 255, 3, 254, 255, 255, 7, 254, 255, 255, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		7: {0, 0, 0, 0, 0, 0, 255, 3, 126, 0, 0, 0, 126, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		2: {0, 0, 0, 0, 0, 0, 0, 0, 254, 255, 255, 7, 254, 255, 255, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		5: {0, 0, 0, 0, 0, 0, 255, 3, 254, 255, 255, 7, 254, 255, 255, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		6: {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	matchClass := func(class uint) bool {
		if (position < len(p.Buffer)) &&
			((classes[class][p.Buffer[position]>>3] & (1 << (p.Buffer[position] & 7))) != 0) {
			position++
			return true
		} else if position >= p.Max {
			p.Max = position
		}
		return false
	}
	peekClass := func(class uint) bool {
		if (position < len(p.Buffer)) &&
			((classes[class][p.Buffer[position]>>3] & (1 << (p.Buffer[position] & 7))) != 0) {
			return true
		}
		return false
	}

	p.rules = [...]func() bool{

		/* 0 Doc <- (StartList (Block { a = cons(yy, a) })* { p.tree = reverse(a) } commit) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
		loop:
			{
				position1 := position
				if !p.rules[ruleBlock]() {
					goto out
				}
				do(0)
				goto loop
			out:
				position = position1
			}
			do(1)
			if !(p.commit(thunkPosition0)) {
				goto ko
			}
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 1 Docblock <- (Block { p.tree = yy } commit) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			if !p.rules[ruleBlock]() {
				goto ko
			}
			do(2)
			if !(p.commit(thunkPosition0)) {
				goto ko
			}
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 2 Block <- (BlankLine* (BlockQuote / Verbatim / Image / CodeBlock / DoctestBlock / Note / Reference / HorizontalRule / HeadingTitle / Heading / Table / DefinitionList / OrderedList / BulletList / HtmlBlock / StyleBlock / Para / Plain)) */
		func() (match bool) {
			position0 := position
		loop:
			if !p.rules[ruleBlankLine]() {
				goto out
			}
			goto loop
		out:
			if !p.rules[ruleBlockQuote]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleVerbatim]() {
				goto nextAlt5
			}
			goto ok
		nextAlt5:
			if !p.rules[ruleImage]() {
				goto nextAlt6
			}
			goto ok
		nextAlt6:
			if !p.rules[ruleCodeBlock]() {
				goto nextAlt7
			}
			goto ok
		nextAlt7:
			if !p.rules[ruleDoctestBlock]() {
				goto nextAlt8
			}
			goto ok
		nextAlt8:
			if !p.rules[ruleNote]() {
				goto nextAlt9
			}
			goto ok
		nextAlt9:
			if !p.rules[ruleReference]() {
				goto nextAlt10
			}
			goto ok
		nextAlt10:
			if !p.rules[ruleHorizontalRule]() {
				goto nextAlt11
			}
			goto ok
		nextAlt11:
			if !p.rules[ruleHeadingTitle]() {
				goto nextAlt12
			}
			goto ok
		nextAlt12:
			if !p.rules[ruleHeading]() {
				goto nextAlt13
			}
			goto ok
		nextAlt13:
			if !p.rules[ruleTable]() {
				goto nextAlt14
			}
			goto ok
		nextAlt14:
			if !p.rules[ruleDefinitionList]() {
				goto nextAlt15
			}
			goto ok
		nextAlt15:
			if !p.rules[ruleOrderedList]() {
				goto nextAlt16
			}
			goto ok
		nextAlt16:
			if !p.rules[ruleBulletList]() {
				goto nextAlt17
			}
			goto ok
		nextAlt17:
			if !p.rules[ruleHtmlBlock]() {
				goto nextAlt18
			}
			goto ok
		nextAlt18:
			if !p.rules[ruleStyleBlock]() {
				goto nextAlt19
			}
			goto ok
		nextAlt19:
			if !p.rules[rulePara]() {
				goto nextAlt20
			}
			goto ok
		nextAlt20:
			if !p.rules[rulePlain]() {
				goto ko
			}
		ok:
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 3 Para <- (NonindentSpace Inlines BlankLine+ { yy = a; yy.key = PARA }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			if !p.rules[ruleInlines]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
		loop:
			if !p.rules[ruleBlankLine]() {
				goto out
			}
			goto loop
		out:
			do(3)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 4 Plain <- (Inlines { yy = a; yy.key = PLAIN }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleInlines]() {
				goto ko
			}
			doarg(yySet, -1)
			do(4)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 5 SetextBottom <- (((&[~] '~') | (&[^] '^') | (&[*] '*') | (&[\-] '-') | (&[=] '='))+ Newline) */
		func() (match bool) {
			position0 := position
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '~':
					position++ // matchChar
				case '^':
					position++ // matchChar
				case '*':
					position++ // matchChar
				case '-':
					position++ // matchChar
				case '=':
					position++ // matchChar
				default:
					goto ko
				}
			}
		loop:
			{
				if position == len(p.Buffer) {
					goto out
				}
				switch p.Buffer[position] {
				case '~':
					position++ // matchChar
				case '^':
					position++ // matchChar
				case '*':
					position++ // matchChar
				case '-':
					position++ // matchChar
				case '=':
					position++ // matchChar
				default:
					goto out
				}
			}
			goto loop
		out:
			if !p.rules[ruleNewline]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 6 HeadingTitle <- (&(SetextBottom RawLine SetextBottom) SetextBottom StartList (!Endline Inline { a = cons(yy, a) })+ Sp Newline SetextBottom { yy = p.mkList(H1TITLE, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			{
				position1 := position
				if !p.rules[ruleSetextBottom]() {
					goto ko
				}
				if !p.rules[ruleRawLine]() {
					goto ko
				}
				if !p.rules[ruleSetextBottom]() {
					goto ko
				}
				position = position1
			}
			if !p.rules[ruleSetextBottom]() {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleEndline]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleInline]() {
				goto ko
			}
			do(5)
		loop:
			{
				position2 := position
				if !p.rules[ruleEndline]() {
					goto ok5
				}
				goto out
			ok5:
				if !p.rules[ruleInline]() {
					goto out
				}
				do(5)
				goto loop
			out:
				position = position2
			}
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			if !p.rules[ruleSetextBottom]() {
				goto ko
			}
			do(6)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 7 Heading <- (&(RawLine SetextBottom) StartList (!Endline Inline { a = cons(yy, a) })+ Sp Newline SetextBottom { yy = p.mkList(getHeadingElm(string(yytext[0])), a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			{
				position1 := position
				if !p.rules[ruleRawLine]() {
					goto ko
				}
				if !p.rules[ruleSetextBottom]() {
					goto ko
				}
				position = position1
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleEndline]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleInline]() {
				goto ko
			}
			do(7)
		loop:
			{
				position2 := position
				if !p.rules[ruleEndline]() {
					goto ok5
				}
				goto out
			ok5:
				if !p.rules[ruleInline]() {
					goto out
				}
				do(7)
				goto loop
			out:
				position = position2
			}
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			if !p.rules[ruleSetextBottom]() {
				goto ko
			}
			do(8)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 8 Image <- (NonindentSpace '.. image:: ' Source BlankLine ((Sp ':alt:' Sp RefSource BlankLine) / (Sp ':target:' Sp Source BlankLine) / (Sp ':align:' Sp Source BlankLine))* {
		    tt := p.mkElem(LIST)
		    title := ""
		    if (a != nil && a.contents.str != "") {
		        title = a.contents.str
		    }
		    tt = p.mkLink(p.mkString(l.contents.str), l.contents.str, title)
		    tt.key = IMAGE

		    if (t != nil) {
		        gg := p.mkLink(p.mkString(t.contents.str), t.contents.str, "")
		        gg.children = tt
		        yy = gg
		    } else {
		        yy = tt
		        yy.children = nil
		    }
		    a = nil
		    t = nil
		    g = nil
		    l = nil
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 4)
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			if !matchString(".. image:: ") {
				goto ko
			}
			if !p.rules[ruleSource]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				{
					position2, thunkPosition2 := position, thunkPosition
					if !p.rules[ruleSp]() {
						goto nextAlt
					}
					if !matchString(":alt:") {
						goto nextAlt
					}
					if !p.rules[ruleSp]() {
						goto nextAlt
					}
					if !p.rules[ruleRefSource]() {
						goto nextAlt
					}
					doarg(yySet, -2)
					if !p.rules[ruleBlankLine]() {
						goto nextAlt
					}
					goto ok
				nextAlt:
					position, thunkPosition = position2, thunkPosition2
					if !p.rules[ruleSp]() {
						goto nextAlt5
					}
					if !matchString(":target:") {
						goto nextAlt5
					}
					if !p.rules[ruleSp]() {
						goto nextAlt5
					}
					if !p.rules[ruleSource]() {
						goto nextAlt5
					}
					doarg(yySet, -3)
					if !p.rules[ruleBlankLine]() {
						goto nextAlt5
					}
					goto ok
				nextAlt5:
					position, thunkPosition = position2, thunkPosition2
					if !p.rules[ruleSp]() {
						goto out
					}
					if !matchString(":align:") {
						goto out
					}
					if !p.rules[ruleSp]() {
						goto out
					}
					if !p.rules[ruleSource]() {
						goto out
					}
					doarg(yySet, -4)
					if !p.rules[ruleBlankLine]() {
						goto out
					}
				}
			ok:
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(9)
			doarg(yyPop, 4)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 9 CodeBlock <- (NonindentSpace '.. code' '-block'? ':: ' Source BlankLine Newline StartList (VerbatimChunk { a = cons(yy, a) })+ {
		    yy = p.mkCodeBlock(a, l.contents.str)
		    l = nil
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			if !matchString(".. code") {
				goto ko
			}
			if !matchString("-block") {
				goto ko1
			}
		ko1:
			if !matchString(":: ") {
				goto ko
			}
			if !p.rules[ruleSource]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -2)
			if !p.rules[ruleVerbatimChunk]() {
				goto ko
			}
			do(10)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleVerbatimChunk]() {
					goto out
				}
				do(10)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(11)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 10 DoctestBlock <- (StartList ((DoctestLine { a = cons(yy, a) })+ (!'>' !BlankLine Line { a = cons(yy, a) })*)+ {
		   yy = p.mkCodeBlock(a, "python")
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleDoctestLine]() {
				goto ko
			}
			do(12)
		loop3:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleDoctestLine]() {
					goto out4
				}
				do(12)
				goto loop3
			out4:
				position, thunkPosition = position1, thunkPosition1
			}
		loop5:
			{
				position2, thunkPosition2 := position, thunkPosition
				if peekChar('>') {
					goto out6
				}
				if !p.rules[ruleBlankLine]() {
					goto ok
				}
				goto out6
			ok:
				if !p.rules[ruleLine]() {
					goto out6
				}
				do(13)
				goto loop5
			out6:
				position, thunkPosition = position2, thunkPosition2
			}
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleDoctestLine]() {
					goto out
				}
				do(12)
			loop8:
				{
					position4, thunkPosition4 := position, thunkPosition
					if !p.rules[ruleDoctestLine]() {
						goto out9
					}
					do(12)
					goto loop8
				out9:
					position, thunkPosition = position4, thunkPosition4
				}
			loop10:
				{
					position5, thunkPosition5 := position, thunkPosition
					if peekChar('>') {
						goto out11
					}
					if !p.rules[ruleBlankLine]() {
						goto ok12
					}
					goto out11
				ok12:
					if !p.rules[ruleLine]() {
						goto out11
					}
					do(13)
					goto loop10
				out11:
					position, thunkPosition = position5, thunkPosition5
				}
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(14)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 11 BlockQuoteRaw <- (':' BlankLine Newline StartList (NonblankIndentedLine { a = cons(yy, a) })+ {
							yy = p.mkStringFromList(a, true)
		                    yy.key = RAW
		                }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !matchChar(':') {
				goto ko
			}
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleNonblankIndentedLine]() {
				goto ko
			}
			do(15)
		loop:
			{
				position1 := position
				if !p.rules[ruleNonblankIndentedLine]() {
					goto out
				}
				do(15)
				goto loop
			out:
				position = position1
			}
			do(16)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 12 BlockQuoteChunk <- (!'::' ':' BlankLine Newline StartList (BlankLine { a = cons(p.mkString("\n"), a) })* (NonblankIndentedLine { a = cons(yy, a) })+ { yy = p.mkStringFromList(a, false) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !matchString("::") {
				goto ok
			}
			goto ko
		ok:
			if !matchChar(':') {
				goto ko
			}
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
		loop:
			{
				position1 := position
				if !p.rules[ruleBlankLine]() {
					goto out
				}
				do(17)
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleNonblankIndentedLine]() {
				goto ko
			}
			do(18)
		loop4:
			{
				position2 := position
				if !p.rules[ruleNonblankIndentedLine]() {
					goto out5
				}
				do(18)
				goto loop4
			out5:
				position = position2
			}
			do(19)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 13 BlockQuote <- (StartList (BlockQuoteChunk { a = cons(yy, a) })+ { yy = p.mkElem(BLOCKQUOTE)
		   yy.children = a }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleBlockQuoteChunk]() {
				goto ko
			}
			do(20)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleBlockQuoteChunk]() {
					goto out
				}
				do(20)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(21)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 14 NonblankIndentedLine <- (!BlankLine IndentedLine) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleBlankLine]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleIndentedLine]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 15 VerbatimChunk <- (StartList (BlankLine { a = cons(p.mkString("\n"), a) })* (NonblankIndentedLine { a = cons(yy, a) })+ { yy = p.mkStringFromList(a, false) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
		loop:
			{
				position1 := position
				if !p.rules[ruleBlankLine]() {
					goto out
				}
				do(22)
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleNonblankIndentedLine]() {
				goto ko
			}
			do(23)
		loop3:
			{
				position2 := position
				if !p.rules[ruleNonblankIndentedLine]() {
					goto out4
				}
				do(23)
				goto loop3
			out4:
				position = position2
			}
			do(24)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 16 Verbatim <- (StartList (VerbatimChunk { a = cons(yy, a) })+ { yy = p.mkStringFromList(a, false)
		   yy.key = VERBATIM }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleVerbatimChunk]() {
				goto ko
			}
			do(25)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleVerbatimChunk]() {
					goto out
				}
				do(25)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(26)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 17 HorizontalRule <- (NonindentSpace ((&[_] ('_' Sp '_' Sp '_' (Sp '_')*)) | (&[~] ('~' Sp '~' Sp '~' (Sp '~')*)) | (&[^] ('^' Sp '^' Sp '^' (Sp '^')*)) | (&[*] ('*' Sp '*' Sp '*' (Sp '*')*)) | (&[\-] ('-' Sp '-' Sp '-' (Sp '-')*)) | (&[=] ('=' Sp '=' Sp '=' (Sp '=')*))) Sp Newline BlankLine+ { yy = p.mkElem(HRULE) }) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '_':
					position++ // matchChar
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('_') {
						goto ko
					}
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('_') {
						goto ko
					}
				loop:
					{
						position1 := position
						if !p.rules[ruleSp]() {
							goto out
						}
						if !matchChar('_') {
							goto out
						}
						goto loop
					out:
						position = position1
					}
				case '~':
					position++ // matchChar
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('~') {
						goto ko
					}
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('~') {
						goto ko
					}
				loop4:
					{
						position2 := position
						if !p.rules[ruleSp]() {
							goto out5
						}
						if !matchChar('~') {
							goto out5
						}
						goto loop4
					out5:
						position = position2
					}
				case '^':
					position++ // matchChar
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('^') {
						goto ko
					}
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('^') {
						goto ko
					}
				loop6:
					{
						position3 := position
						if !p.rules[ruleSp]() {
							goto out7
						}
						if !matchChar('^') {
							goto out7
						}
						goto loop6
					out7:
						position = position3
					}
				case '*':
					position++ // matchChar
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('*') {
						goto ko
					}
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('*') {
						goto ko
					}
				loop8:
					{
						position4 := position
						if !p.rules[ruleSp]() {
							goto out9
						}
						if !matchChar('*') {
							goto out9
						}
						goto loop8
					out9:
						position = position4
					}
				case '-':
					position++ // matchChar
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('-') {
						goto ko
					}
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('-') {
						goto ko
					}
				loop10:
					{
						position5 := position
						if !p.rules[ruleSp]() {
							goto out11
						}
						if !matchChar('-') {
							goto out11
						}
						goto loop10
					out11:
						position = position5
					}
				case '=':
					position++ // matchChar
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('=') {
						goto ko
					}
					if !p.rules[ruleSp]() {
						goto ko
					}
					if !matchChar('=') {
						goto ko
					}
				loop12:
					{
						position6 := position
						if !p.rules[ruleSp]() {
							goto out13
						}
						if !matchChar('=') {
							goto out13
						}
						goto loop12
					out13:
						position = position6
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
		loop14:
			if !p.rules[ruleBlankLine]() {
				goto out15
			}
			goto loop14
		out15:
			do(27)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 18 Table <- (GridTable / HeaderLessGridTable / SimpleTable) */
		func() (match bool) {
			if !p.rules[ruleGridTable]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleHeaderLessGridTable]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !p.rules[ruleSimpleTable]() {
				return
			}
		ok:
			match = true
			return
		},
		/* 19 SimpleTable <- ('NotImplemented' 'simpleTable') */
		func() (match bool) {
			position0 := position
			if !matchString("NotImplemented") {
				goto ko
			}
			if !matchString("simpleTable") {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 20 GridTable <- (StartList (GridTableHeader { yy.key = TABLEHEAD; a = cons(yy, a) }) (GridTableHeaderSep { a = yy }) (GridTableBody { a = cons(yy, a) })+ {
		    yy = p.mkList(TABLE, a)
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleGridTableHeader]() {
				goto ko
			}
			do(28)
			if !p.rules[ruleGridTableHeaderSep]() {
				goto ko
			}
			do(29)
			if !p.rules[ruleGridTableBody]() {
				goto ko
			}
			do(30)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleGridTableBody]() {
					goto out
				}
				do(30)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(31)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 21 HeaderLessGridTable <- (StartList (GridTableSep (GridTableBody { a = cons(yy, a) })+) {
		    yy = p.mkList(TABLE, a)
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleGridTableSep]() {
				goto ko
			}
			if !p.rules[ruleGridTableBody]() {
				goto ko
			}
			do(32)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleGridTableBody]() {
					goto out
				}
				do(32)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(33)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 22 GridTableHeader <- (Sp '+' ('-'+ '+')+ BlankLine StartList < (GridTableRow { a = cons(yy, a) })+ > {
			yy = p.mkList(TABLEBODY, a)
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !matchChar('+') {
				goto ko
			}
			if !matchChar('-') {
				goto ko
			}
		loop3:
			if !matchChar('-') {
				goto out4
			}
			goto loop3
		out4:
			if !matchChar('+') {
				goto ko
			}
		loop:
			{
				position1 := position
				if !matchChar('-') {
					goto out
				}
			loop5:
				if !matchChar('-') {
					goto out6
				}
				goto loop5
			out6:
				if !matchChar('+') {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			begin = position
			if !p.rules[ruleGridTableRow]() {
				goto ko
			}
			do(34)
		loop7:
			{
				position2, thunkPosition2 := position, thunkPosition
				if !p.rules[ruleGridTableRow]() {
					goto out8
				}
				do(34)
				goto loop7
			out8:
				position, thunkPosition = position2, thunkPosition2
			}
			end = position
			do(35)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 23 GridTableBody <- (StartList (GridTableRow { a = cons(yy, a) } GridTableSep)+ { yy = p.mkList(TABLEBODY, a);}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleGridTableRow]() {
				goto ko
			}
			do(36)
			if !p.rules[ruleGridTableSep]() {
				goto ko
			}
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleGridTableRow]() {
					goto out
				}
				do(36)
				if !p.rules[ruleGridTableSep]() {
					goto out
				}
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(37)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 24 GridTableRow <- (Sp '|' Sp StartList (< TableCell > Sp '|' {
			// TODO: support inline text
			raw := p.mkString(yytext)
			raw.key = RAW
			yy = p.mkElem(TABLECELL)
			yy.children = raw
			a = cons(yy, a)
		})+ BlankLine {
			yy = p.mkList(TABLEROW, a)
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !matchChar('|') {
				goto ko
			}
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			begin = position
			if !p.rules[ruleTableCell]() {
				goto ko
			}
			end = position
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !matchChar('|') {
				goto ko
			}
			do(38)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				begin = position
				if !p.rules[ruleTableCell]() {
					goto out
				}
				end = position
				if !p.rules[ruleSp]() {
					goto out
				}
				if !matchChar('|') {
					goto out
				}
				do(38)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			do(39)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 25 TableCell <- (StartList (Spacechar / ((&[\t ] Sp) | (&[\\] EscapedChar) | (&[\-] '-') | (&[/] '/') | (&[<] '<') | (&[>] '>') | (&[:] ':') | (&[0-9A-Za-z\200-\377] Alphanumeric)))+ {
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleSpacechar]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '\t', ' ':
					if !p.rules[ruleSp]() {
						goto ko
					}
				case '\\':
					if !p.rules[ruleEscapedChar]() {
						goto ko
					}
				case '-':
					position++ // matchChar
				case '/':
					position++ // matchChar
				case '<':
					position++ // matchChar
				case '>':
					position++ // matchChar
				case ':':
					position++ // matchChar
				default:
					if !p.rules[ruleAlphanumeric]() {
						goto ko
					}
				}
			}
		ok:
		loop:
			if !p.rules[ruleSpacechar]() {
				goto nextAlt7
			}
			goto ok6
		nextAlt7:
			{
				if position == len(p.Buffer) {
					goto out
				}
				switch p.Buffer[position] {
				case '\t', ' ':
					if !p.rules[ruleSp]() {
						goto out
					}
				case '\\':
					if !p.rules[ruleEscapedChar]() {
						goto out
					}
				case '-':
					position++ // matchChar
				case '/':
					position++ // matchChar
				case '<':
					position++ // matchChar
				case '>':
					position++ // matchChar
				case ':':
					position++ // matchChar
				default:
					if !p.rules[ruleAlphanumeric]() {
						goto out
					}
				}
			}
		ok6:
			goto loop
		out:
			do(40)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 26 GridTableHeaderSep <- (Sp '+' ('='+ '+')+ BlankLine) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !matchChar('+') {
				goto ko
			}
			if !matchChar('=') {
				goto ko
			}
		loop3:
			if !matchChar('=') {
				goto out4
			}
			goto loop3
		out4:
			if !matchChar('+') {
				goto ko
			}
		loop:
			{
				position1 := position
				if !matchChar('=') {
					goto out
				}
			loop5:
				if !matchChar('=') {
					goto out6
				}
				goto loop5
			out6:
				if !matchChar('+') {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 27 GridTableSep <- (Sp '+' ('-'+ '+')+ BlankLine) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !matchChar('+') {
				goto ko
			}
			if !matchChar('-') {
				goto ko
			}
		loop3:
			if !matchChar('-') {
				goto out4
			}
			goto loop3
		out4:
			if !matchChar('+') {
				goto ko
			}
		loop:
			{
				position1 := position
				if !matchChar('-') {
					goto out
				}
			loop5:
				if !matchChar('-') {
					goto out6
				}
				goto loop5
			out6:
				if !matchChar('+') {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 28 Bullet <- (!HorizontalRule NonindentSpace ((&[\-] '-') | (&[*] '*') | (&[+] '+')) Spacechar+) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			if !p.rules[ruleHorizontalRule]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '-':
					position++ // matchChar
				case '*':
					position++ // matchChar
				case '+':
					position++ // matchChar
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpacechar]() {
				goto ko
			}
		loop:
			if !p.rules[ruleSpacechar]() {
				goto out
			}
			goto loop
		out:
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 29 BulletList <- (&Bullet (ListTight / ListLoose) { yy.key = BULLETLIST }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			{
				position1 := position
				if !p.rules[ruleBullet]() {
					goto ko
				}
				position = position1
			}
			if !p.rules[ruleListTight]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleListLoose]() {
				goto ko
			}
		ok:
			do(41)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 30 ListTight <- (StartList (ListItemTight { a = cons(yy, a) })+ BlankLine* !((&[:~] DefMarker) | (&[*+\-] Bullet) | (&[#0-9] Enumerator)) { yy = p.mkList(LIST, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleListItemTight]() {
				goto ko
			}
			do(42)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleListItemTight]() {
					goto out
				}
				do(42)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
		loop3:
			if !p.rules[ruleBlankLine]() {
				goto out4
			}
			goto loop3
		out4:
			{
				if position == len(p.Buffer) {
					goto ok
				}
				switch p.Buffer[position] {
				case ':', '~':
					if !p.rules[ruleDefMarker]() {
						goto ok
					}
				case '*', '+', '-':
					if !p.rules[ruleBullet]() {
						goto ok
					}
				default:
					if !p.rules[ruleEnumerator]() {
						goto ok
					}
				}
			}
			goto ko
		ok:
			do(43)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 31 ListLoose <- (StartList (ListItem BlankLine* {
		    li := b.children
		    li.contents.str += "\n\n"
		    a = cons(b, a)
		})+ { yy = p.mkList(LIST, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleListItem]() {
				goto ko
			}
			doarg(yySet, -2)
		loop3:
			if !p.rules[ruleBlankLine]() {
				goto out4
			}
			goto loop3
		out4:
			do(44)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleListItem]() {
					goto out
				}
				doarg(yySet, -2)
			loop5:
				if !p.rules[ruleBlankLine]() {
					goto out6
				}
				goto loop5
			out6:
				do(44)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(45)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 32 ListItem <- (((&[:~] DefMarker) | (&[*+\-] Bullet) | (&[#0-9] Enumerator)) StartList ListBlock { a = cons(yy, a) } (ListContinuationBlock { a = cons(yy, a) })* {
		   raw := p.mkStringFromList(a, false)
		   raw.key = RAW
		   yy = p.mkElem(LISTITEM)
		   yy.children = raw
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case ':', '~':
					if !p.rules[ruleDefMarker]() {
						goto ko
					}
				case '*', '+', '-':
					if !p.rules[ruleBullet]() {
						goto ko
					}
				default:
					if !p.rules[ruleEnumerator]() {
						goto ko
					}
				}
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleListBlock]() {
				goto ko
			}
			do(46)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleListContinuationBlock]() {
					goto out
				}
				do(47)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(48)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 33 ListItemTight <- (((&[:~] DefMarker) | (&[*+\-] Bullet) | (&[#0-9] Enumerator)) StartList ListBlock { a = cons(yy, a) } (!BlankLine ListContinuationBlock { a = cons(yy, a) })* !ListContinuationBlock {
		   raw := p.mkStringFromList(a, false)
		   raw.key = RAW
		   yy = p.mkElem(LISTITEM)
		   yy.children = raw
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case ':', '~':
					if !p.rules[ruleDefMarker]() {
						goto ko
					}
				case '*', '+', '-':
					if !p.rules[ruleBullet]() {
						goto ko
					}
				default:
					if !p.rules[ruleEnumerator]() {
						goto ko
					}
				}
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleListBlock]() {
				goto ko
			}
			do(49)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleBlankLine]() {
					goto ok4
				}
				goto out
			ok4:
				if !p.rules[ruleListContinuationBlock]() {
					goto out
				}
				do(50)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			if !p.rules[ruleListContinuationBlock]() {
				goto ok5
			}
			goto ko
		ok5:
			do(51)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 34 ListBlock <- (StartList !BlankLine Line { a = cons(yy, a) } (ListBlockLine { a = cons(yy, a) })* { yy = p.mkStringFromList(a, false) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleBlankLine]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleLine]() {
				goto ko
			}
			do(52)
		loop:
			{
				position1 := position
				if !p.rules[ruleListBlockLine]() {
					goto out
				}
				do(53)
				goto loop
			out:
				position = position1
			}
			do(54)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 35 ListContinuationBlock <- (StartList (< BlankLine* > {   if len(yytext) == 0 {
		         a = cons(p.mkString("\001"), a) // block separator
		    } else {
		         a = cons(p.mkString(yytext), a)
		    }
		}) (Indent ListBlock { a = cons(yy, a) })+ {  yy = p.mkStringFromList(a, false) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			begin = position
		loop:
			if !p.rules[ruleBlankLine]() {
				goto out
			}
			goto loop
		out:
			end = position
			do(55)
			if !p.rules[ruleIndent]() {
				goto ko
			}
			if !p.rules[ruleListBlock]() {
				goto ko
			}
			do(56)
		loop3:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleIndent]() {
					goto out4
				}
				if !p.rules[ruleListBlock]() {
					goto out4
				}
				do(56)
				goto loop3
			out4:
				position, thunkPosition = position1, thunkPosition1
			}
			do(57)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 36 Enumerator <- (NonindentSpace ((&[#] '#'+) | (&[0-9] [0-9]+)) '.' Spacechar+) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '#':
					if !matchChar('#') {
						goto ko
					}
				loop:
					if !matchChar('#') {
						goto out
					}
					goto loop
				out:
					break
				default:
					if !matchClass(0) {
						goto ko
					}
				loop4:
					if !matchClass(0) {
						goto out5
					}
					goto loop4
				out5:
				}
			}
			if !matchChar('.') {
				goto ko
			}
			if !p.rules[ruleSpacechar]() {
				goto ko
			}
		loop6:
			if !p.rules[ruleSpacechar]() {
				goto out7
			}
			goto loop6
		out7:
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 37 OrderedList <- (&Enumerator (ListTight / ListLoose) { yy.key = ORDEREDLIST }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			{
				position1 := position
				if !p.rules[ruleEnumerator]() {
					goto ko
				}
				position = position1
			}
			if !p.rules[ruleListTight]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleListLoose]() {
				goto ko
			}
		ok:
			do(58)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 38 ListBlockLine <- (!BlankLine !((&[:~] DefMarker) | (&[\t #*+\-0-9] (Indent? ((&[*+\-] Bullet) | (&[#0-9] Enumerator))))) !HorizontalRule OptionallyIndentedLine) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			if !p.rules[ruleBlankLine]() {
				goto ok
			}
			goto ko
		ok:
			{
				position1 := position
				{
					if position == len(p.Buffer) {
						goto ok2
					}
					switch p.Buffer[position] {
					case ':', '~':
						if !p.rules[ruleDefMarker]() {
							goto ok2
						}
					default:
						if !p.rules[ruleIndent]() {
							goto ko4
						}
					ko4:
						{
							if position == len(p.Buffer) {
								goto ok2
							}
							switch p.Buffer[position] {
							case '*', '+', '-':
								if !p.rules[ruleBullet]() {
									goto ok2
								}
							default:
								if !p.rules[ruleEnumerator]() {
									goto ok2
								}
							}
						}
					}
				}
				goto ko
			ok2:
				position = position1
			}
			if !p.rules[ruleHorizontalRule]() {
				goto ok7
			}
			goto ko
		ok7:
			if !p.rules[ruleOptionallyIndentedLine]() {
				goto ko
			}
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 39 HtmlBlockOpenAddress <- ('<' Spnl ((&[A] 'ADDRESS') | (&[a] 'address')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'A':
					position++
					if !matchString("DDRESS") {
						goto ko
					}
				case 'a':
					position++
					if !matchString("ddress") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 40 HtmlBlockCloseAddress <- ('<' Spnl '/' ((&[A] 'ADDRESS') | (&[a] 'address')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'A':
					position++
					if !matchString("DDRESS") {
						goto ko
					}
				case 'a':
					position++
					if !matchString("ddress") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 41 HtmlBlockAddress <- (HtmlBlockOpenAddress (HtmlBlockAddress / (!HtmlBlockCloseAddress .))* HtmlBlockCloseAddress) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenAddress]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockAddress]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseAddress]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseAddress]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 42 HtmlBlockOpenBlockquote <- ('<' Spnl ((&[B] 'BLOCKQUOTE') | (&[b] 'blockquote')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'B':
					position++
					if !matchString("LOCKQUOTE") {
						goto ko
					}
				case 'b':
					position++
					if !matchString("lockquote") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 43 HtmlBlockCloseBlockquote <- ('<' Spnl '/' ((&[B] 'BLOCKQUOTE') | (&[b] 'blockquote')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'B':
					position++
					if !matchString("LOCKQUOTE") {
						goto ko
					}
				case 'b':
					position++
					if !matchString("lockquote") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 44 HtmlBlockBlockquote <- (HtmlBlockOpenBlockquote (HtmlBlockBlockquote / (!HtmlBlockCloseBlockquote .))* HtmlBlockCloseBlockquote) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenBlockquote]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockBlockquote]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseBlockquote]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseBlockquote]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 45 HtmlBlockOpenCenter <- ('<' Spnl ((&[C] 'CENTER') | (&[c] 'center')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'C':
					position++
					if !matchString("ENTER") {
						goto ko
					}
				case 'c':
					position++
					if !matchString("enter") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 46 HtmlBlockCloseCenter <- ('<' Spnl '/' ((&[C] 'CENTER') | (&[c] 'center')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'C':
					position++
					if !matchString("ENTER") {
						goto ko
					}
				case 'c':
					position++
					if !matchString("enter") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 47 HtmlBlockCenter <- (HtmlBlockOpenCenter (HtmlBlockCenter / (!HtmlBlockCloseCenter .))* HtmlBlockCloseCenter) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenCenter]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockCenter]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseCenter]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseCenter]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 48 HtmlBlockOpenDir <- ('<' Spnl ((&[D] 'DIR') | (&[d] 'dir')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++
					if !matchString("IR") {
						goto ko
					}
				case 'd':
					position++
					if !matchString("ir") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 49 HtmlBlockCloseDir <- ('<' Spnl '/' ((&[D] 'DIR') | (&[d] 'dir')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++
					if !matchString("IR") {
						goto ko
					}
				case 'd':
					position++
					if !matchString("ir") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 50 HtmlBlockDir <- (HtmlBlockOpenDir (HtmlBlockDir / (!HtmlBlockCloseDir .))* HtmlBlockCloseDir) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenDir]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockDir]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseDir]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseDir]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 51 HtmlBlockOpenDiv <- ('<' Spnl ((&[D] 'DIV') | (&[d] 'div')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++
					if !matchString("IV") {
						goto ko
					}
				case 'd':
					position++
					if !matchString("iv") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 52 HtmlBlockCloseDiv <- ('<' Spnl '/' ((&[D] 'DIV') | (&[d] 'div')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++
					if !matchString("IV") {
						goto ko
					}
				case 'd':
					position++
					if !matchString("iv") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 53 HtmlBlockDiv <- (HtmlBlockOpenDiv (HtmlBlockDiv / (!HtmlBlockCloseDiv .))* HtmlBlockCloseDiv) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenDiv]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockDiv]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseDiv]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseDiv]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 54 HtmlBlockOpenDl <- ('<' Spnl ((&[D] 'DL') | (&[d] 'dl')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++ // matchString(`DL`)
					if !matchChar('L') {
						goto ko
					}
				case 'd':
					position++ // matchString(`dl`)
					if !matchChar('l') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 55 HtmlBlockCloseDl <- ('<' Spnl '/' ((&[D] 'DL') | (&[d] 'dl')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++ // matchString(`DL`)
					if !matchChar('L') {
						goto ko
					}
				case 'd':
					position++ // matchString(`dl`)
					if !matchChar('l') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 56 HtmlBlockDl <- (HtmlBlockOpenDl (HtmlBlockDl / (!HtmlBlockCloseDl .))* HtmlBlockCloseDl) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenDl]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockDl]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseDl]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseDl]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 57 HtmlBlockOpenFieldset <- ('<' Spnl ((&[F] 'FIELDSET') | (&[f] 'fieldset')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'F':
					position++
					if !matchString("IELDSET") {
						goto ko
					}
				case 'f':
					position++
					if !matchString("ieldset") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 58 HtmlBlockCloseFieldset <- ('<' Spnl '/' ((&[F] 'FIELDSET') | (&[f] 'fieldset')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'F':
					position++
					if !matchString("IELDSET") {
						goto ko
					}
				case 'f':
					position++
					if !matchString("ieldset") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 59 HtmlBlockFieldset <- (HtmlBlockOpenFieldset (HtmlBlockFieldset / (!HtmlBlockCloseFieldset .))* HtmlBlockCloseFieldset) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenFieldset]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockFieldset]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseFieldset]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseFieldset]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 60 HtmlBlockOpenForm <- ('<' Spnl ((&[F] 'FORM') | (&[f] 'form')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'F':
					position++
					if !matchString("ORM") {
						goto ko
					}
				case 'f':
					position++
					if !matchString("orm") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 61 HtmlBlockCloseForm <- ('<' Spnl '/' ((&[F] 'FORM') | (&[f] 'form')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'F':
					position++
					if !matchString("ORM") {
						goto ko
					}
				case 'f':
					position++
					if !matchString("orm") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 62 HtmlBlockForm <- (HtmlBlockOpenForm (HtmlBlockForm / (!HtmlBlockCloseForm .))* HtmlBlockCloseForm) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenForm]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockForm]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseForm]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseForm]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 63 HtmlBlockOpenH1 <- ('<' Spnl ((&[H] 'H1') | (&[h] 'h1')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H1`)
					if !matchChar('1') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h1`)
					if !matchChar('1') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 64 HtmlBlockCloseH1 <- ('<' Spnl '/' ((&[H] 'H1') | (&[h] 'h1')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H1`)
					if !matchChar('1') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h1`)
					if !matchChar('1') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 65 HtmlBlockH1 <- (HtmlBlockOpenH1 (HtmlBlockH1 / (!HtmlBlockCloseH1 .))* HtmlBlockCloseH1) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenH1]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockH1]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseH1]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseH1]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 66 HtmlBlockOpenH2 <- ('<' Spnl ((&[H] 'H2') | (&[h] 'h2')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H2`)
					if !matchChar('2') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h2`)
					if !matchChar('2') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 67 HtmlBlockCloseH2 <- ('<' Spnl '/' ((&[H] 'H2') | (&[h] 'h2')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H2`)
					if !matchChar('2') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h2`)
					if !matchChar('2') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 68 HtmlBlockH2 <- (HtmlBlockOpenH2 (HtmlBlockH2 / (!HtmlBlockCloseH2 .))* HtmlBlockCloseH2) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenH2]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockH2]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseH2]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseH2]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 69 HtmlBlockOpenH3 <- ('<' Spnl ((&[H] 'H3') | (&[h] 'h3')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H3`)
					if !matchChar('3') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h3`)
					if !matchChar('3') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 70 HtmlBlockCloseH3 <- ('<' Spnl '/' ((&[H] 'H3') | (&[h] 'h3')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H3`)
					if !matchChar('3') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h3`)
					if !matchChar('3') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 71 HtmlBlockH3 <- (HtmlBlockOpenH3 (HtmlBlockH3 / (!HtmlBlockCloseH3 .))* HtmlBlockCloseH3) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenH3]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockH3]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseH3]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseH3]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 72 HtmlBlockOpenH4 <- ('<' Spnl ((&[H] 'H4') | (&[h] 'h4')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H4`)
					if !matchChar('4') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h4`)
					if !matchChar('4') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 73 HtmlBlockCloseH4 <- ('<' Spnl '/' ((&[H] 'H4') | (&[h] 'h4')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H4`)
					if !matchChar('4') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h4`)
					if !matchChar('4') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 74 HtmlBlockH4 <- (HtmlBlockOpenH4 (HtmlBlockH4 / (!HtmlBlockCloseH4 .))* HtmlBlockCloseH4) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenH4]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockH4]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseH4]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseH4]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 75 HtmlBlockOpenH5 <- ('<' Spnl ((&[H] 'H5') | (&[h] 'h5')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H5`)
					if !matchChar('5') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h5`)
					if !matchChar('5') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 76 HtmlBlockCloseH5 <- ('<' Spnl '/' ((&[H] 'H5') | (&[h] 'h5')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H5`)
					if !matchChar('5') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h5`)
					if !matchChar('5') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 77 HtmlBlockH5 <- (HtmlBlockOpenH5 (HtmlBlockH5 / (!HtmlBlockCloseH5 .))* HtmlBlockCloseH5) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenH5]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockH5]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseH5]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseH5]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 78 HtmlBlockOpenH6 <- ('<' Spnl ((&[H] 'H6') | (&[h] 'h6')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H6`)
					if !matchChar('6') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h6`)
					if !matchChar('6') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 79 HtmlBlockCloseH6 <- ('<' Spnl '/' ((&[H] 'H6') | (&[h] 'h6')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++ // matchString(`H6`)
					if !matchChar('6') {
						goto ko
					}
				case 'h':
					position++ // matchString(`h6`)
					if !matchChar('6') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 80 HtmlBlockH6 <- (HtmlBlockOpenH6 (HtmlBlockH6 / (!HtmlBlockCloseH6 .))* HtmlBlockCloseH6) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenH6]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockH6]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseH6]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseH6]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 81 HtmlBlockOpenMenu <- ('<' Spnl ((&[M] 'MENU') | (&[m] 'menu')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'M':
					position++
					if !matchString("ENU") {
						goto ko
					}
				case 'm':
					position++
					if !matchString("enu") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 82 HtmlBlockCloseMenu <- ('<' Spnl '/' ((&[M] 'MENU') | (&[m] 'menu')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'M':
					position++
					if !matchString("ENU") {
						goto ko
					}
				case 'm':
					position++
					if !matchString("enu") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 83 HtmlBlockMenu <- (HtmlBlockOpenMenu (HtmlBlockMenu / (!HtmlBlockCloseMenu .))* HtmlBlockCloseMenu) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenMenu]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockMenu]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseMenu]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseMenu]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 84 HtmlBlockOpenNoframes <- ('<' Spnl ((&[N] 'NOFRAMES') | (&[n] 'noframes')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'N':
					position++
					if !matchString("OFRAMES") {
						goto ko
					}
				case 'n':
					position++
					if !matchString("oframes") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 85 HtmlBlockCloseNoframes <- ('<' Spnl '/' ((&[N] 'NOFRAMES') | (&[n] 'noframes')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'N':
					position++
					if !matchString("OFRAMES") {
						goto ko
					}
				case 'n':
					position++
					if !matchString("oframes") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 86 HtmlBlockNoframes <- (HtmlBlockOpenNoframes (HtmlBlockNoframes / (!HtmlBlockCloseNoframes .))* HtmlBlockCloseNoframes) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenNoframes]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockNoframes]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseNoframes]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseNoframes]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 87 HtmlBlockOpenNoscript <- ('<' Spnl ((&[N] 'NOSCRIPT') | (&[n] 'noscript')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'N':
					position++
					if !matchString("OSCRIPT") {
						goto ko
					}
				case 'n':
					position++
					if !matchString("oscript") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 88 HtmlBlockCloseNoscript <- ('<' Spnl '/' ((&[N] 'NOSCRIPT') | (&[n] 'noscript')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'N':
					position++
					if !matchString("OSCRIPT") {
						goto ko
					}
				case 'n':
					position++
					if !matchString("oscript") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 89 HtmlBlockNoscript <- (HtmlBlockOpenNoscript (HtmlBlockNoscript / (!HtmlBlockCloseNoscript .))* HtmlBlockCloseNoscript) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenNoscript]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockNoscript]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseNoscript]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseNoscript]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 90 HtmlBlockOpenOl <- ('<' Spnl ((&[O] 'OL') | (&[o] 'ol')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'O':
					position++ // matchString(`OL`)
					if !matchChar('L') {
						goto ko
					}
				case 'o':
					position++ // matchString(`ol`)
					if !matchChar('l') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 91 HtmlBlockCloseOl <- ('<' Spnl '/' ((&[O] 'OL') | (&[o] 'ol')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'O':
					position++ // matchString(`OL`)
					if !matchChar('L') {
						goto ko
					}
				case 'o':
					position++ // matchString(`ol`)
					if !matchChar('l') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 92 HtmlBlockOl <- (HtmlBlockOpenOl (HtmlBlockOl / (!HtmlBlockCloseOl .))* HtmlBlockCloseOl) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenOl]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockOl]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseOl]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseOl]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 93 HtmlBlockOpenP <- ('<' Spnl ((&[P] 'P') | (&[p] 'p')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'P':
					position++ // matchChar
				case 'p':
					position++ // matchChar
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 94 HtmlBlockCloseP <- ('<' Spnl '/' ((&[P] 'P') | (&[p] 'p')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'P':
					position++ // matchChar
				case 'p':
					position++ // matchChar
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 95 HtmlBlockP <- (HtmlBlockOpenP (HtmlBlockP / (!HtmlBlockCloseP .))* HtmlBlockCloseP) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenP]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockP]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseP]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseP]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 96 HtmlBlockOpenPre <- ('<' Spnl ((&[P] 'PRE') | (&[p] 'pre')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'P':
					position++
					if !matchString("RE") {
						goto ko
					}
				case 'p':
					position++
					if !matchString("re") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 97 HtmlBlockClosePre <- ('<' Spnl '/' ((&[P] 'PRE') | (&[p] 'pre')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'P':
					position++
					if !matchString("RE") {
						goto ko
					}
				case 'p':
					position++
					if !matchString("re") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 98 HtmlBlockPre <- (HtmlBlockOpenPre (HtmlBlockPre / (!HtmlBlockClosePre .))* HtmlBlockClosePre) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenPre]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockPre]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockClosePre]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockClosePre]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 99 HtmlBlockOpenTable <- ('<' Spnl ((&[T] 'TABLE') | (&[t] 'table')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++
					if !matchString("ABLE") {
						goto ko
					}
				case 't':
					position++
					if !matchString("able") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 100 HtmlBlockCloseTable <- ('<' Spnl '/' ((&[T] 'TABLE') | (&[t] 'table')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++
					if !matchString("ABLE") {
						goto ko
					}
				case 't':
					position++
					if !matchString("able") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 101 HtmlBlockTable <- (HtmlBlockOpenTable (HtmlBlockTable / (!HtmlBlockCloseTable .))* HtmlBlockCloseTable) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenTable]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockTable]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseTable]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseTable]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 102 HtmlBlockOpenUl <- ('<' Spnl ((&[U] 'UL') | (&[u] 'ul')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'U':
					position++ // matchString(`UL`)
					if !matchChar('L') {
						goto ko
					}
				case 'u':
					position++ // matchString(`ul`)
					if !matchChar('l') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 103 HtmlBlockCloseUl <- ('<' Spnl '/' ((&[U] 'UL') | (&[u] 'ul')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'U':
					position++ // matchString(`UL`)
					if !matchChar('L') {
						goto ko
					}
				case 'u':
					position++ // matchString(`ul`)
					if !matchChar('l') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 104 HtmlBlockUl <- (HtmlBlockOpenUl (HtmlBlockUl / (!HtmlBlockCloseUl .))* HtmlBlockCloseUl) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenUl]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockUl]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseUl]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseUl]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 105 HtmlBlockOpenDd <- ('<' Spnl ((&[D] 'DD') | (&[d] 'dd')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++ // matchString(`DD`)
					if !matchChar('D') {
						goto ko
					}
				case 'd':
					position++ // matchString(`dd`)
					if !matchChar('d') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 106 HtmlBlockCloseDd <- ('<' Spnl '/' ((&[D] 'DD') | (&[d] 'dd')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++ // matchString(`DD`)
					if !matchChar('D') {
						goto ko
					}
				case 'd':
					position++ // matchString(`dd`)
					if !matchChar('d') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 107 HtmlBlockDd <- (HtmlBlockOpenDd (HtmlBlockDd / (!HtmlBlockCloseDd .))* HtmlBlockCloseDd) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenDd]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockDd]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseDd]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseDd]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 108 HtmlBlockOpenDt <- ('<' Spnl ((&[D] 'DT') | (&[d] 'dt')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++ // matchString(`DT`)
					if !matchChar('T') {
						goto ko
					}
				case 'd':
					position++ // matchString(`dt`)
					if !matchChar('t') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 109 HtmlBlockCloseDt <- ('<' Spnl '/' ((&[D] 'DT') | (&[d] 'dt')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'D':
					position++ // matchString(`DT`)
					if !matchChar('T') {
						goto ko
					}
				case 'd':
					position++ // matchString(`dt`)
					if !matchChar('t') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 110 HtmlBlockDt <- (HtmlBlockOpenDt (HtmlBlockDt / (!HtmlBlockCloseDt .))* HtmlBlockCloseDt) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenDt]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockDt]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseDt]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseDt]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 111 HtmlBlockOpenFrameset <- ('<' Spnl ((&[F] 'FRAMESET') | (&[f] 'frameset')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'F':
					position++
					if !matchString("RAMESET") {
						goto ko
					}
				case 'f':
					position++
					if !matchString("rameset") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 112 HtmlBlockCloseFrameset <- ('<' Spnl '/' ((&[F] 'FRAMESET') | (&[f] 'frameset')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'F':
					position++
					if !matchString("RAMESET") {
						goto ko
					}
				case 'f':
					position++
					if !matchString("rameset") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 113 HtmlBlockFrameset <- (HtmlBlockOpenFrameset (HtmlBlockFrameset / (!HtmlBlockCloseFrameset .))* HtmlBlockCloseFrameset) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenFrameset]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockFrameset]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseFrameset]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseFrameset]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 114 HtmlBlockOpenLi <- ('<' Spnl ((&[L] 'LI') | (&[l] 'li')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'L':
					position++ // matchString(`LI`)
					if !matchChar('I') {
						goto ko
					}
				case 'l':
					position++ // matchString(`li`)
					if !matchChar('i') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 115 HtmlBlockCloseLi <- ('<' Spnl '/' ((&[L] 'LI') | (&[l] 'li')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'L':
					position++ // matchString(`LI`)
					if !matchChar('I') {
						goto ko
					}
				case 'l':
					position++ // matchString(`li`)
					if !matchChar('i') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 116 HtmlBlockLi <- (HtmlBlockOpenLi (HtmlBlockLi / (!HtmlBlockCloseLi .))* HtmlBlockCloseLi) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenLi]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockLi]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseLi]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseLi]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 117 HtmlBlockOpenTbody <- ('<' Spnl ((&[T] 'TBODY') | (&[t] 'tbody')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++
					if !matchString("BODY") {
						goto ko
					}
				case 't':
					position++
					if !matchString("body") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 118 HtmlBlockCloseTbody <- ('<' Spnl '/' ((&[T] 'TBODY') | (&[t] 'tbody')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++
					if !matchString("BODY") {
						goto ko
					}
				case 't':
					position++
					if !matchString("body") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 119 HtmlBlockTbody <- (HtmlBlockOpenTbody (HtmlBlockTbody / (!HtmlBlockCloseTbody .))* HtmlBlockCloseTbody) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenTbody]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockTbody]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseTbody]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseTbody]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 120 HtmlBlockOpenTd <- ('<' Spnl ((&[T] 'TD') | (&[t] 'td')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++ // matchString(`TD`)
					if !matchChar('D') {
						goto ko
					}
				case 't':
					position++ // matchString(`td`)
					if !matchChar('d') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 121 HtmlBlockCloseTd <- ('<' Spnl '/' ((&[T] 'TD') | (&[t] 'td')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++ // matchString(`TD`)
					if !matchChar('D') {
						goto ko
					}
				case 't':
					position++ // matchString(`td`)
					if !matchChar('d') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 122 HtmlBlockTd <- (HtmlBlockOpenTd (HtmlBlockTd / (!HtmlBlockCloseTd .))* HtmlBlockCloseTd) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenTd]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockTd]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseTd]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseTd]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 123 HtmlBlockOpenTfoot <- ('<' Spnl ((&[T] 'TFOOT') | (&[t] 'tfoot')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++
					if !matchString("FOOT") {
						goto ko
					}
				case 't':
					position++
					if !matchString("foot") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 124 HtmlBlockCloseTfoot <- ('<' Spnl '/' ((&[T] 'TFOOT') | (&[t] 'tfoot')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++
					if !matchString("FOOT") {
						goto ko
					}
				case 't':
					position++
					if !matchString("foot") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 125 HtmlBlockTfoot <- (HtmlBlockOpenTfoot (HtmlBlockTfoot / (!HtmlBlockCloseTfoot .))* HtmlBlockCloseTfoot) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenTfoot]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockTfoot]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseTfoot]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseTfoot]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 126 HtmlBlockOpenTh <- ('<' Spnl ((&[T] 'TH') | (&[t] 'th')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++ // matchString(`TH`)
					if !matchChar('H') {
						goto ko
					}
				case 't':
					position++ // matchString(`th`)
					if !matchChar('h') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 127 HtmlBlockCloseTh <- ('<' Spnl '/' ((&[T] 'TH') | (&[t] 'th')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++ // matchString(`TH`)
					if !matchChar('H') {
						goto ko
					}
				case 't':
					position++ // matchString(`th`)
					if !matchChar('h') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 128 HtmlBlockTh <- (HtmlBlockOpenTh (HtmlBlockTh / (!HtmlBlockCloseTh .))* HtmlBlockCloseTh) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenTh]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockTh]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseTh]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseTh]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 129 HtmlBlockOpenThead <- ('<' Spnl ((&[T] 'THEAD') | (&[t] 'thead')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++
					if !matchString("HEAD") {
						goto ko
					}
				case 't':
					position++
					if !matchString("head") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 130 HtmlBlockCloseThead <- ('<' Spnl '/' ((&[T] 'THEAD') | (&[t] 'thead')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++
					if !matchString("HEAD") {
						goto ko
					}
				case 't':
					position++
					if !matchString("head") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 131 HtmlBlockThead <- (HtmlBlockOpenThead (HtmlBlockThead / (!HtmlBlockCloseThead .))* HtmlBlockCloseThead) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenThead]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockThead]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseThead]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseThead]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 132 HtmlBlockOpenTr <- ('<' Spnl ((&[T] 'TR') | (&[t] 'tr')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++ // matchString(`TR`)
					if !matchChar('R') {
						goto ko
					}
				case 't':
					position++ // matchString(`tr`)
					if !matchChar('r') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 133 HtmlBlockCloseTr <- ('<' Spnl '/' ((&[T] 'TR') | (&[t] 'tr')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'T':
					position++ // matchString(`TR`)
					if !matchChar('R') {
						goto ko
					}
				case 't':
					position++ // matchString(`tr`)
					if !matchChar('r') {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 134 HtmlBlockTr <- (HtmlBlockOpenTr (HtmlBlockTr / (!HtmlBlockCloseTr .))* HtmlBlockCloseTr) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenTr]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockTr]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleHtmlBlockCloseTr]() {
					goto ok5
				}
				goto out
			ok5:
				if !matchDot() {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseTr]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 135 HtmlBlockOpenScript <- ('<' Spnl ((&[S] 'SCRIPT') | (&[s] 'script')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'S':
					position++
					if !matchString("CRIPT") {
						goto ko
					}
				case 's':
					position++
					if !matchString("cript") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 136 HtmlBlockCloseScript <- ('<' Spnl '/' ((&[S] 'SCRIPT') | (&[s] 'script')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'S':
					position++
					if !matchString("CRIPT") {
						goto ko
					}
				case 's':
					position++
					if !matchString("cript") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 137 HtmlBlockScript <- (HtmlBlockOpenScript (!HtmlBlockCloseScript .)* HtmlBlockCloseScript) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenScript]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockCloseScript]() {
					goto ok
				}
				goto out
			ok:
				if !matchDot() {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseScript]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 138 HtmlBlockOpenHead <- ('<' Spnl ((&[H] 'HEAD') | (&[h] 'head')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++
					if !matchString("EAD") {
						goto ko
					}
				case 'h':
					position++
					if !matchString("ead") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 139 HtmlBlockCloseHead <- ('<' Spnl '/' ((&[H] 'HEAD') | (&[h] 'head')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'H':
					position++
					if !matchString("EAD") {
						goto ko
					}
				case 'h':
					position++
					if !matchString("ead") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 140 HtmlBlockHead <- (HtmlBlockOpenHead (!HtmlBlockCloseHead .)* HtmlBlockCloseHead) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHtmlBlockOpenHead]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleHtmlBlockCloseHead]() {
					goto ok
				}
				goto out
			ok:
				if !matchDot() {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleHtmlBlockCloseHead]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 141 HtmlBlockInTags <- (HtmlBlockAddress / HtmlBlockBlockquote / HtmlBlockCenter / HtmlBlockDir / HtmlBlockDiv / HtmlBlockDl / HtmlBlockFieldset / HtmlBlockForm / HtmlBlockH1 / HtmlBlockH2 / HtmlBlockH3 / HtmlBlockH4 / HtmlBlockH5 / HtmlBlockH6 / HtmlBlockMenu / HtmlBlockNoframes / HtmlBlockNoscript / HtmlBlockOl / HtmlBlockP / HtmlBlockPre / HtmlBlockTable / HtmlBlockUl / HtmlBlockDd / HtmlBlockDt / HtmlBlockFrameset / HtmlBlockLi / HtmlBlockTbody / HtmlBlockTd / HtmlBlockTfoot / HtmlBlockTh / HtmlBlockThead / HtmlBlockTr / HtmlBlockScript / HtmlBlockHead) */
		func() (match bool) {
			if !p.rules[ruleHtmlBlockAddress]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleHtmlBlockBlockquote]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !p.rules[ruleHtmlBlockCenter]() {
				goto nextAlt4
			}
			goto ok
		nextAlt4:
			if !p.rules[ruleHtmlBlockDir]() {
				goto nextAlt5
			}
			goto ok
		nextAlt5:
			if !p.rules[ruleHtmlBlockDiv]() {
				goto nextAlt6
			}
			goto ok
		nextAlt6:
			if !p.rules[ruleHtmlBlockDl]() {
				goto nextAlt7
			}
			goto ok
		nextAlt7:
			if !p.rules[ruleHtmlBlockFieldset]() {
				goto nextAlt8
			}
			goto ok
		nextAlt8:
			if !p.rules[ruleHtmlBlockForm]() {
				goto nextAlt9
			}
			goto ok
		nextAlt9:
			if !p.rules[ruleHtmlBlockH1]() {
				goto nextAlt10
			}
			goto ok
		nextAlt10:
			if !p.rules[ruleHtmlBlockH2]() {
				goto nextAlt11
			}
			goto ok
		nextAlt11:
			if !p.rules[ruleHtmlBlockH3]() {
				goto nextAlt12
			}
			goto ok
		nextAlt12:
			if !p.rules[ruleHtmlBlockH4]() {
				goto nextAlt13
			}
			goto ok
		nextAlt13:
			if !p.rules[ruleHtmlBlockH5]() {
				goto nextAlt14
			}
			goto ok
		nextAlt14:
			if !p.rules[ruleHtmlBlockH6]() {
				goto nextAlt15
			}
			goto ok
		nextAlt15:
			if !p.rules[ruleHtmlBlockMenu]() {
				goto nextAlt16
			}
			goto ok
		nextAlt16:
			if !p.rules[ruleHtmlBlockNoframes]() {
				goto nextAlt17
			}
			goto ok
		nextAlt17:
			if !p.rules[ruleHtmlBlockNoscript]() {
				goto nextAlt18
			}
			goto ok
		nextAlt18:
			if !p.rules[ruleHtmlBlockOl]() {
				goto nextAlt19
			}
			goto ok
		nextAlt19:
			if !p.rules[ruleHtmlBlockP]() {
				goto nextAlt20
			}
			goto ok
		nextAlt20:
			if !p.rules[ruleHtmlBlockPre]() {
				goto nextAlt21
			}
			goto ok
		nextAlt21:
			if !p.rules[ruleHtmlBlockTable]() {
				goto nextAlt22
			}
			goto ok
		nextAlt22:
			if !p.rules[ruleHtmlBlockUl]() {
				goto nextAlt23
			}
			goto ok
		nextAlt23:
			if !p.rules[ruleHtmlBlockDd]() {
				goto nextAlt24
			}
			goto ok
		nextAlt24:
			if !p.rules[ruleHtmlBlockDt]() {
				goto nextAlt25
			}
			goto ok
		nextAlt25:
			if !p.rules[ruleHtmlBlockFrameset]() {
				goto nextAlt26
			}
			goto ok
		nextAlt26:
			if !p.rules[ruleHtmlBlockLi]() {
				goto nextAlt27
			}
			goto ok
		nextAlt27:
			if !p.rules[ruleHtmlBlockTbody]() {
				goto nextAlt28
			}
			goto ok
		nextAlt28:
			if !p.rules[ruleHtmlBlockTd]() {
				goto nextAlt29
			}
			goto ok
		nextAlt29:
			if !p.rules[ruleHtmlBlockTfoot]() {
				goto nextAlt30
			}
			goto ok
		nextAlt30:
			if !p.rules[ruleHtmlBlockTh]() {
				goto nextAlt31
			}
			goto ok
		nextAlt31:
			if !p.rules[ruleHtmlBlockThead]() {
				goto nextAlt32
			}
			goto ok
		nextAlt32:
			if !p.rules[ruleHtmlBlockTr]() {
				goto nextAlt33
			}
			goto ok
		nextAlt33:
			if !p.rules[ruleHtmlBlockScript]() {
				goto nextAlt34
			}
			goto ok
		nextAlt34:
			if !p.rules[ruleHtmlBlockHead]() {
				return
			}
		ok:
			match = true
			return
		},
		/* 142 HtmlBlock <- (&'<' < (HtmlBlockInTags / HtmlComment / HtmlBlockSelfClosing) > BlankLine+ {   if p.extension.FilterHTML {
		        yy = p.mkList(LIST, nil)
		    } else {
		        yy = p.mkString(yytext)
		        yy.key = HTMLBLOCK
		    }
		}) */
		func() (match bool) {
			position0 := position
			if !peekChar('<') {
				goto ko
			}
			begin = position
			if !p.rules[ruleHtmlBlockInTags]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleHtmlComment]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !p.rules[ruleHtmlBlockSelfClosing]() {
				goto ko
			}
		ok:
			end = position
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
		loop:
			if !p.rules[ruleBlankLine]() {
				goto out
			}
			goto loop
		out:
			do(59)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 143 HtmlBlockSelfClosing <- ('<' Spnl HtmlBlockType Spnl HtmlAttribute* '/' Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !p.rules[ruleHtmlBlockType]() {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('/') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 144 HtmlBlockType <- ('dir' / 'div' / 'dl' / 'fieldset' / 'form' / 'h1' / 'h2' / 'h3' / 'h4' / 'h5' / 'h6' / 'noframes' / 'p' / 'table' / 'dd' / 'tbody' / 'td' / 'tfoot' / 'th' / 'thead' / 'DIR' / 'DIV' / 'DL' / 'FIELDSET' / 'FORM' / 'H1' / 'H2' / 'H3' / 'H4' / 'H5' / 'H6' / 'NOFRAMES' / 'P' / 'TABLE' / 'DD' / 'TBODY' / 'TD' / 'TFOOT' / 'TH' / 'THEAD' / ((&[S] 'SCRIPT') | (&[T] 'TR') | (&[L] 'LI') | (&[F] 'FRAMESET') | (&[D] 'DT') | (&[U] 'UL') | (&[P] 'PRE') | (&[O] 'OL') | (&[N] 'NOSCRIPT') | (&[M] 'MENU') | (&[I] 'ISINDEX') | (&[H] 'HR') | (&[C] 'CENTER') | (&[B] 'BLOCKQUOTE') | (&[A] 'ADDRESS') | (&[s] 'script') | (&[t] 'tr') | (&[l] 'li') | (&[f] 'frameset') | (&[d] 'dt') | (&[u] 'ul') | (&[p] 'pre') | (&[o] 'ol') | (&[n] 'noscript') | (&[m] 'menu') | (&[i] 'isindex') | (&[h] 'hr') | (&[c] 'center') | (&[b] 'blockquote') | (&[a] 'address'))) */
		func() (match bool) {
			if !matchString("dir") {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !matchString("div") {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !matchString("dl") {
				goto nextAlt4
			}
			goto ok
		nextAlt4:
			if !matchString("fieldset") {
				goto nextAlt5
			}
			goto ok
		nextAlt5:
			if !matchString("form") {
				goto nextAlt6
			}
			goto ok
		nextAlt6:
			if !matchString("h1") {
				goto nextAlt7
			}
			goto ok
		nextAlt7:
			if !matchString("h2") {
				goto nextAlt8
			}
			goto ok
		nextAlt8:
			if !matchString("h3") {
				goto nextAlt9
			}
			goto ok
		nextAlt9:
			if !matchString("h4") {
				goto nextAlt10
			}
			goto ok
		nextAlt10:
			if !matchString("h5") {
				goto nextAlt11
			}
			goto ok
		nextAlt11:
			if !matchString("h6") {
				goto nextAlt12
			}
			goto ok
		nextAlt12:
			if !matchString("noframes") {
				goto nextAlt13
			}
			goto ok
		nextAlt13:
			if !matchChar('p') {
				goto nextAlt14
			}
			goto ok
		nextAlt14:
			if !matchString("table") {
				goto nextAlt15
			}
			goto ok
		nextAlt15:
			if !matchString("dd") {
				goto nextAlt16
			}
			goto ok
		nextAlt16:
			if !matchString("tbody") {
				goto nextAlt17
			}
			goto ok
		nextAlt17:
			if !matchString("td") {
				goto nextAlt18
			}
			goto ok
		nextAlt18:
			if !matchString("tfoot") {
				goto nextAlt19
			}
			goto ok
		nextAlt19:
			if !matchString("th") {
				goto nextAlt20
			}
			goto ok
		nextAlt20:
			if !matchString("thead") {
				goto nextAlt21
			}
			goto ok
		nextAlt21:
			if !matchString("DIR") {
				goto nextAlt22
			}
			goto ok
		nextAlt22:
			if !matchString("DIV") {
				goto nextAlt23
			}
			goto ok
		nextAlt23:
			if !matchString("DL") {
				goto nextAlt24
			}
			goto ok
		nextAlt24:
			if !matchString("FIELDSET") {
				goto nextAlt25
			}
			goto ok
		nextAlt25:
			if !matchString("FORM") {
				goto nextAlt26
			}
			goto ok
		nextAlt26:
			if !matchString("H1") {
				goto nextAlt27
			}
			goto ok
		nextAlt27:
			if !matchString("H2") {
				goto nextAlt28
			}
			goto ok
		nextAlt28:
			if !matchString("H3") {
				goto nextAlt29
			}
			goto ok
		nextAlt29:
			if !matchString("H4") {
				goto nextAlt30
			}
			goto ok
		nextAlt30:
			if !matchString("H5") {
				goto nextAlt31
			}
			goto ok
		nextAlt31:
			if !matchString("H6") {
				goto nextAlt32
			}
			goto ok
		nextAlt32:
			if !matchString("NOFRAMES") {
				goto nextAlt33
			}
			goto ok
		nextAlt33:
			if !matchChar('P') {
				goto nextAlt34
			}
			goto ok
		nextAlt34:
			if !matchString("TABLE") {
				goto nextAlt35
			}
			goto ok
		nextAlt35:
			if !matchString("DD") {
				goto nextAlt36
			}
			goto ok
		nextAlt36:
			if !matchString("TBODY") {
				goto nextAlt37
			}
			goto ok
		nextAlt37:
			if !matchString("TD") {
				goto nextAlt38
			}
			goto ok
		nextAlt38:
			if !matchString("TFOOT") {
				goto nextAlt39
			}
			goto ok
		nextAlt39:
			if !matchString("TH") {
				goto nextAlt40
			}
			goto ok
		nextAlt40:
			if !matchString("THEAD") {
				goto nextAlt41
			}
			goto ok
		nextAlt41:
			{
				if position == len(p.Buffer) {
					return
				}
				switch p.Buffer[position] {
				case 'S':
					position++
					if !matchString("CRIPT") {
						return
					}
				case 'T':
					position++ // matchString(`TR`)
					if !matchChar('R') {
						return
					}
				case 'L':
					position++ // matchString(`LI`)
					if !matchChar('I') {
						return
					}
				case 'F':
					position++
					if !matchString("RAMESET") {
						return
					}
				case 'D':
					position++ // matchString(`DT`)
					if !matchChar('T') {
						return
					}
				case 'U':
					position++ // matchString(`UL`)
					if !matchChar('L') {
						return
					}
				case 'P':
					position++
					if !matchString("RE") {
						return
					}
				case 'O':
					position++ // matchString(`OL`)
					if !matchChar('L') {
						return
					}
				case 'N':
					position++
					if !matchString("OSCRIPT") {
						return
					}
				case 'M':
					position++
					if !matchString("ENU") {
						return
					}
				case 'I':
					position++
					if !matchString("SINDEX") {
						return
					}
				case 'H':
					position++ // matchString(`HR`)
					if !matchChar('R') {
						return
					}
				case 'C':
					position++
					if !matchString("ENTER") {
						return
					}
				case 'B':
					position++
					if !matchString("LOCKQUOTE") {
						return
					}
				case 'A':
					position++
					if !matchString("DDRESS") {
						return
					}
				case 's':
					position++
					if !matchString("cript") {
						return
					}
				case 't':
					position++ // matchString(`tr`)
					if !matchChar('r') {
						return
					}
				case 'l':
					position++ // matchString(`li`)
					if !matchChar('i') {
						return
					}
				case 'f':
					position++
					if !matchString("rameset") {
						return
					}
				case 'd':
					position++ // matchString(`dt`)
					if !matchChar('t') {
						return
					}
				case 'u':
					position++ // matchString(`ul`)
					if !matchChar('l') {
						return
					}
				case 'p':
					position++
					if !matchString("re") {
						return
					}
				case 'o':
					position++ // matchString(`ol`)
					if !matchChar('l') {
						return
					}
				case 'n':
					position++
					if !matchString("oscript") {
						return
					}
				case 'm':
					position++
					if !matchString("enu") {
						return
					}
				case 'i':
					position++
					if !matchString("sindex") {
						return
					}
				case 'h':
					position++ // matchString(`hr`)
					if !matchChar('r') {
						return
					}
				case 'c':
					position++
					if !matchString("enter") {
						return
					}
				case 'b':
					position++
					if !matchString("lockquote") {
						return
					}
				case 'a':
					position++
					if !matchString("ddress") {
						return
					}
				default:
					return
				}
			}
		ok:
			match = true
			return
		},
		/* 145 StyleOpen <- ('<' Spnl ((&[S] 'STYLE') | (&[s] 'style')) Spnl HtmlAttribute* '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'S':
					position++
					if !matchString("TYLE") {
						goto ko
					}
				case 's':
					position++
					if !matchString("tyle") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop:
			if !p.rules[ruleHtmlAttribute]() {
				goto out
			}
			goto loop
		out:
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 146 StyleClose <- ('<' Spnl '/' ((&[S] 'STYLE') | (&[s] 'style')) Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('/') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case 'S':
					position++
					if !matchString("TYLE") {
						goto ko
					}
				case 's':
					position++
					if !matchString("tyle") {
						goto ko
					}
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 147 InStyleTags <- (StyleOpen (!StyleClose .)* StyleClose) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleStyleOpen]() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleStyleClose]() {
					goto ok
				}
				goto out
			ok:
				if !matchDot() {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			if !p.rules[ruleStyleClose]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 148 StyleBlock <- (< InStyleTags > BlankLine* {   if p.extension.FilterStyles {
		        yy = p.mkList(LIST, nil)
		    } else {
		        yy = p.mkString(yytext)
		        yy.key = HTMLBLOCK
		    }
		}) */
		func() (match bool) {
			position0 := position
			begin = position
			if !p.rules[ruleInStyleTags]() {
				goto ko
			}
			end = position
		loop:
			if !p.rules[ruleBlankLine]() {
				goto out
			}
			goto loop
		out:
			do(60)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 149 Inlines <- (StartList ((!Endline Inline { a = cons(yy, a) }) / (Endline &Inline { a = cons(c, a) }))+ Endline? {
			yy = p.mkList(LIST, a)
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			{
				position1 := position
				if !p.rules[ruleEndline]() {
					goto ok5
				}
				goto nextAlt
			ok5:
				if !p.rules[ruleInline]() {
					goto nextAlt
				}
				do(61)
				goto ok
			nextAlt:
				position = position1
				if !p.rules[ruleEndline]() {
					goto ko
				}
				doarg(yySet, -2)
				{
					position2 := position
					if !p.rules[ruleInline]() {
						goto ko
					}
					position = position2
				}
				do(62)
			}
		ok:
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				{
					position4 := position
					if !p.rules[ruleEndline]() {
						goto ok9
					}
					goto nextAlt8
				ok9:
					if !p.rules[ruleInline]() {
						goto nextAlt8
					}
					do(61)
					goto ok7
				nextAlt8:
					position = position4
					if !p.rules[ruleEndline]() {
						goto out
					}
					doarg(yySet, -2)
					{
						position5 := position
						if !p.rules[ruleInline]() {
							goto out
						}
						position = position5
					}
					do(62)
				}
			ok7:
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			if !p.rules[ruleEndline]() {
				goto ko11
			}
		ko11:
			do(63)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 150 Inline <- (Link / Str / Endline / UlOrStarLine / Space / Strong / Emph / Strike / NoteReference / Footnote / Code / ApplicationDepent / RawHtml / Entity / EscapedChar / Smart / Symbol) */
		func() (match bool) {
			if !p.rules[ruleLink]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleStr]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !p.rules[ruleEndline]() {
				goto nextAlt4
			}
			goto ok
		nextAlt4:
			if !p.rules[ruleUlOrStarLine]() {
				goto nextAlt5
			}
			goto ok
		nextAlt5:
			if !p.rules[ruleSpace]() {
				goto nextAlt6
			}
			goto ok
		nextAlt6:
			if !p.rules[ruleStrong]() {
				goto nextAlt7
			}
			goto ok
		nextAlt7:
			if !p.rules[ruleEmph]() {
				goto nextAlt8
			}
			goto ok
		nextAlt8:
			if !p.rules[ruleStrike]() {
				goto nextAlt9
			}
			goto ok
		nextAlt9:
			if !p.rules[ruleNoteReference]() {
				goto nextAlt10
			}
			goto ok
		nextAlt10:
			if !p.rules[ruleFootnote]() {
				goto nextAlt11
			}
			goto ok
		nextAlt11:
			if !p.rules[ruleCode]() {
				goto nextAlt12
			}
			goto ok
		nextAlt12:
			if !p.rules[ruleApplicationDepent]() {
				goto nextAlt13
			}
			goto ok
		nextAlt13:
			if !p.rules[ruleRawHtml]() {
				goto nextAlt14
			}
			goto ok
		nextAlt14:
			if !p.rules[ruleEntity]() {
				goto nextAlt15
			}
			goto ok
		nextAlt15:
			if !p.rules[ruleEscapedChar]() {
				goto nextAlt16
			}
			goto ok
		nextAlt16:
			if !p.rules[ruleSmart]() {
				goto nextAlt17
			}
			goto ok
		nextAlt17:
			if !p.rules[ruleSymbol]() {
				return
			}
		ok:
			match = true
			return
		},
		/* 151 Space <- (Spacechar+ { yy = p.mkString(" ")
		   yy.key = SPACE }) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleSpacechar]() {
				goto ko
			}
		loop:
			if !p.rules[ruleSpacechar]() {
				goto out
			}
			goto loop
		out:
			do(64)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 152 Str <- (StartList < NormalChar+ > { a = cons(p.mkString(yytext), a) } (StrChunk { a = cons(yy, a) })* { if a.next == nil { yy = a; } else { yy = p.mkList(LIST, a) } }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			begin = position
			if !p.rules[ruleNormalChar]() {
				goto ko
			}
		loop:
			if !p.rules[ruleNormalChar]() {
				goto out
			}
			goto loop
		out:
			end = position
			do(65)
		loop3:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleStrChunk]() {
					goto out4
				}
				do(66)
				goto loop3
			out4:
				position, thunkPosition = position1, thunkPosition1
			}
			do(67)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 153 StrChunk <- ((< (NormalChar / ('_'+ &Alphanumeric))+ > { yy = p.mkString(yytext) }) / AposChunk) */
		func() (match bool) {
			position0 := position
			{
				position1 := position
				begin = position
				if !p.rules[ruleNormalChar]() {
					goto nextAlt6
				}
				goto ok5
			nextAlt6:
				if !matchChar('_') {
					goto nextAlt
				}
			loop7:
				if !matchChar('_') {
					goto out8
				}
				goto loop7
			out8:
				{
					position2 := position
					if !p.rules[ruleAlphanumeric]() {
						goto nextAlt
					}
					position = position2
				}
			ok5:
			loop:
				{
					position2 := position
					if !p.rules[ruleNormalChar]() {
						goto nextAlt11
					}
					goto ok10
				nextAlt11:
					if !matchChar('_') {
						goto out
					}
				loop12:
					if !matchChar('_') {
						goto out13
					}
					goto loop12
				out13:
					{
						position4 := position
						if !p.rules[ruleAlphanumeric]() {
							goto out
						}
						position = position4
					}
				ok10:
					goto loop
				out:
					position = position2
				}
				end = position
				do(68)
				goto ok
			nextAlt:
				position = position1
				if !p.rules[ruleAposChunk]() {
					goto ko
				}
			}
		ok:
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 154 AposChunk <- (&{p.extension.Smart} '\'' &Alphanumeric { yy = p.mkElem(APOSTROPHE) }) */
		func() (match bool) {
			position0 := position
			if !(p.extension.Smart) {
				goto ko
			}
			if !matchChar('\'') {
				goto ko
			}
			{
				position1 := position
				if !p.rules[ruleAlphanumeric]() {
					goto ko
				}
				position = position1
			}
			do(69)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 155 EscapedChar <- ('\\' !Newline < [-\\`|*_{}[\]()#+.!><] > { yy = p.mkString(yytext) }) */
		func() (match bool) {
			position0 := position
			if !matchChar('\\') {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ok
			}
			goto ko
		ok:
			begin = position
			if !matchClass(1) {
				goto ko
			}
			end = position
			do(70)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 156 Entity <- ((HexEntity / DecEntity / CharEntity) { yy = p.mkString(yytext); yy.key = HTML }) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleHexEntity]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleDecEntity]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !p.rules[ruleCharEntity]() {
				goto ko
			}
		ok:
			do(71)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 157 Endline <- (LineBreak / TerminalEndline / NormalEndline) */
		func() (match bool) {
			if !p.rules[ruleLineBreak]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleTerminalEndline]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !p.rules[ruleNormalEndline]() {
				return
			}
		ok:
			match = true
			return
		},
		/* 158 NormalEndline <- (Sp Newline !BlankLine !'>' !(Line ((&[\-] '-'+) | (&[=] '='+)) Newline) { yy = p.mkString("\n")
		   yy.key = SPACE }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			if !p.rules[ruleBlankLine]() {
				goto ok
			}
			goto ko
		ok:
			if peekChar('>') {
				goto ko
			}
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleLine]() {
					goto ok2
				}
				{
					if position == len(p.Buffer) {
						goto ok2
					}
					switch p.Buffer[position] {
					case '-':
						if !matchChar('-') {
							goto ok2
						}
					loop:
						if !matchChar('-') {
							goto out
						}
						goto loop
					out:
						break
					case '=':
						if !matchChar('=') {
							goto ok2
						}
					loop6:
						if !matchChar('=') {
							goto out7
						}
						goto loop6
					out7:
						break
					default:
						goto ok2
					}
				}
				if !p.rules[ruleNewline]() {
					goto ok2
				}
				goto ko
			ok2:
				position, thunkPosition = position1, thunkPosition1
			}
			do(72)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 159 TerminalEndline <- (Sp Newline !. { yy = nil }) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			if position < len(p.Buffer) {
				goto ko
			}
			do(73)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 160 LineBreak <- ('  ' NormalEndline { yy = p.mkElem(LINEBREAK) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			if !matchString("  ") {
				goto ko
			}
			if !p.rules[ruleNormalEndline]() {
				goto ko
			}
			do(74)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 161 Symbol <- (< SpecialChar > { yy = p.mkString(yytext) }) */
		func() (match bool) {
			position0 := position
			begin = position
			if !p.rules[ruleSpecialChar]() {
				goto ko
			}
			end = position
			do(75)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 162 ApplicationDepent <- (!'`_' !'``_' '`' !'``' QuotedRefSource '`' !'``' !'_' { yy = p.mkString(yytext); a = nil }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !matchString("`_") {
				goto ok
			}
			goto ko
		ok:
			if !matchString("``_") {
				goto ok2
			}
			goto ko
		ok2:
			if !matchChar('`') {
				goto ko
			}
			if !matchString("``") {
				goto ok3
			}
			goto ko
		ok3:
			if !p.rules[ruleQuotedRefSource]() {
				goto ko
			}
			doarg(yySet, -1)
			if !matchChar('`') {
				goto ko
			}
			if !matchString("``") {
				goto ok4
			}
			goto ko
		ok4:
			if peekChar('_') {
				goto ko
			}
			do(76)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 163 UlOrStarLine <- ((UlLine / StarLine) { yy = p.mkString(yytext) }) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleUlLine]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleStarLine]() {
				goto ko
			}
		ok:
			do(77)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 164 StarLine <- ((&[*] (< '****' '*'* >)) | (&[\t ] (< Spacechar '*'+ &Spacechar >))) */
		func() (match bool) {
			position0 := position
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '*':
					begin = position
					if !matchString("****") {
						goto ko
					}
				loop:
					if !matchChar('*') {
						goto out
					}
					goto loop
				out:
					end = position
				case '\t', ' ':
					begin = position
					if !p.rules[ruleSpacechar]() {
						goto ko
					}
					if !matchChar('*') {
						goto ko
					}
				loop4:
					if !matchChar('*') {
						goto out5
					}
					goto loop4
				out5:
					{
						position1 := position
						if !p.rules[ruleSpacechar]() {
							goto ko
						}
						position = position1
					}
					end = position
				default:
					goto ko
				}
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 165 UlLine <- ((&[_] (< '____' '_'* >)) | (&[\t ] (< Spacechar '_'+ &Spacechar >))) */
		func() (match bool) {
			position0 := position
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '_':
					begin = position
					if !matchString("____") {
						goto ko
					}
				loop:
					if !matchChar('_') {
						goto out
					}
					goto loop
				out:
					end = position
				case '\t', ' ':
					begin = position
					if !p.rules[ruleSpacechar]() {
						goto ko
					}
					if !matchChar('_') {
						goto ko
					}
				loop4:
					if !matchChar('_') {
						goto out5
					}
					goto loop4
				out5:
					{
						position1 := position
						if !p.rules[ruleSpacechar]() {
							goto ko
						}
						position = position1
					}
					end = position
				default:
					goto ko
				}
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 166 Whitespace <- ((&[\n\r] Newline) | (&[\t ] Spacechar)) */
		func() (match bool) {
			{
				if position == len(p.Buffer) {
					return
				}
				switch p.Buffer[position] {
				case '\n', '\r':
					if !p.rules[ruleNewline]() {
						return
					}
				case '\t', ' ':
					if !p.rules[ruleSpacechar]() {
						return
					}
				default:
					return
				}
			}
			match = true
			return
		},
		/* 167 Emph <- ('*' !Whitespace StartList (!'*' Inline { a = cons(b, a) })+ '*' { yy = p.mkList(EMPH, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !matchChar('*') {
				goto ko
			}
			if !p.rules[ruleWhitespace]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if peekChar('*') {
				goto ko
			}
			if !p.rules[ruleInline]() {
				goto ko
			}
			doarg(yySet, -2)
			do(78)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if peekChar('*') {
					goto out
				}
				if !p.rules[ruleInline]() {
					goto out
				}
				doarg(yySet, -2)
				do(78)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			if !matchChar('*') {
				goto ko
			}
			do(79)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 168 Strong <- ('**' !Whitespace StartList (!'**' Inline { a = cons(b, a) })+ '**' { yy = p.mkList(STRONG, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !matchString("**") {
				goto ko
			}
			if !p.rules[ruleWhitespace]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !matchString("**") {
				goto ok4
			}
			goto ko
		ok4:
			if !p.rules[ruleInline]() {
				goto ko
			}
			doarg(yySet, -2)
			do(80)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !matchString("**") {
					goto ok5
				}
				goto out
			ok5:
				if !p.rules[ruleInline]() {
					goto out
				}
				doarg(yySet, -2)
				do(80)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			if !matchString("**") {
				goto ko
			}
			do(81)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 169 Strike <- (&{p.extension.Strike} '~~' !Whitespace StartList (!'~~' Inline { a = cons(b, a) })+ '~~' { yy = p.mkList(STRIKE, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !(p.extension.Strike) {
				goto ko
			}
			if !matchString("~~") {
				goto ko
			}
			if !p.rules[ruleWhitespace]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !matchString("~~") {
				goto ok4
			}
			goto ko
		ok4:
			if !p.rules[ruleInline]() {
				goto ko
			}
			doarg(yySet, -2)
			do(82)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !matchString("~~") {
					goto ok5
				}
				goto out
			ok5:
				if !p.rules[ruleInline]() {
					goto out
				}
				doarg(yySet, -2)
				do(82)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			if !matchString("~~") {
				goto ko
			}
			do(83)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 170 Link <- (ReferenceLink / ExplicitLink / AutoLink) */
		func() (match bool) {
			if !p.rules[ruleReferenceLink]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleExplicitLink]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !p.rules[ruleAutoLink]() {
				return
			}
		ok:
			match = true
			return
		},
		/* 171 ReferenceLink <- (UnquotedRefLinkUnderbar / QuotedRefLinkUnderbar) */
		func() (match bool) {
			if !p.rules[ruleUnquotedRefLinkUnderbar]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleQuotedRefLinkUnderbar]() {
				return
			}
		ok:
			match = true
			return
		},
		/* 172 UnquotedRefLinkUnderbar <- (< UnquotedLinkSource > '_' {
		    if match, found := p.findReference(p.mkString(a.contents.str)); found {
		        yy = p.mkLink(p.mkString(a.contents.str), match.url, match.title)
		        a = nil
		    } else {
		        result := p.mkElem(LIST)
		        result.children = cons(p.mkString(a.contents.str), cons(a, p.mkString(yytext)));
		        yy = result
		    }
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			begin = position
			if !p.rules[ruleUnquotedLinkSource]() {
				goto ko
			}
			doarg(yySet, -1)
			end = position
			if !matchChar('_') {
				goto ko
			}
			do(84)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 173 QuotedRefLinkUnderbar <- (!'`_' !'``_' '`' !'``' QuotedRefSource ('`' !'``') '_' {
		    if match, found := p.findReference(p.mkString(a.contents.str)); found {
		        yy = p.mkLink(p.mkString(yytext), match.url, match.title)
		        a = nil
		    } else {
		        result := p.mkElem(LIST)
		        result.children = cons(p.mkString(a.contents.str), cons(a, p.mkString(yytext)));
		        yy = result
		    }
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !matchString("`_") {
				goto ok
			}
			goto ko
		ok:
			if !matchString("``_") {
				goto ok2
			}
			goto ko
		ok2:
			if !matchChar('`') {
				goto ko
			}
			if !matchString("``") {
				goto ok3
			}
			goto ko
		ok3:
			if !p.rules[ruleQuotedRefSource]() {
				goto ko
			}
			doarg(yySet, -1)
			if !matchChar('`') {
				goto ko
			}
			if !matchString("``") {
				goto ok4
			}
			goto ko
		ok4:
			if !matchChar('_') {
				goto ko
			}
			do(85)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 174 ExplicitLink <- (Label '(' Sp Source Spnl Title Sp ')' {
		   yy = p.mkLink(l.children, s.contents.str, t.contents.str)
		   s = nil
		   t = nil
		   l = nil
		 }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 3)
			if !p.rules[ruleLabel]() {
				goto ko
			}
			doarg(yySet, -2)
			if !matchChar('(') {
				goto ko
			}
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleSource]() {
				goto ko
			}
			doarg(yySet, -3)
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !p.rules[ruleTitle]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !matchChar(')') {
				goto ko
			}
			do(86)
			doarg(yyPop, 3)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 175 Source <- (< SourceContents > { yy = p.mkString(yytext) }) */
		func() (match bool) {
			position0 := position
			begin = position
			if !p.rules[ruleSourceContents]() {
				goto ko
			}
			end = position
			do(87)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 176 SourceContents <- ((!'(' !')' !'>' Nonspacechar)+ / ('(' SourceContents ')'))* */
		func() (match bool) {
		loop:
			{
				position1 := position
				if position == len(p.Buffer) {
					goto nextAlt
				}
				switch p.Buffer[position] {
				case '(', ')', '>':
					goto nextAlt
				default:
					if !p.rules[ruleNonspacechar]() {
						goto nextAlt
					}
				}
			loop5:
				if position == len(p.Buffer) {
					goto out6
				}
				switch p.Buffer[position] {
				case '(', ')', '>':
					goto out6
				default:
					if !p.rules[ruleNonspacechar]() {
						goto out6
					}
				}
				goto loop5
			out6:
				goto ok
			nextAlt:
				if !matchChar('(') {
					goto out
				}
				if !p.rules[ruleSourceContents]() {
					goto out
				}
				if !matchChar(')') {
					goto out
				}
			ok:
				goto loop
			out:
				position = position1
			}
			match = true
			return
		},
		/* 177 Title <- ((TitleSingle / TitleDouble / (< '' >)) { yy = p.mkString(yytext) }) */
		func() (match bool) {
			if !p.rules[ruleTitleSingle]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleTitleDouble]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			begin = position
			end = position
		ok:
			do(88)
			match = true
			return
		},
		/* 178 TitleSingle <- ('\'' < (!('\'' Sp ((&[)] ')') | (&[\n\r] Newline))) .)* > '\'') */
		func() (match bool) {
			position0 := position
			if !matchChar('\'') {
				goto ko
			}
			begin = position
		loop:
			{
				position1 := position
				{
					position2 := position
					if !matchChar('\'') {
						goto ok
					}
					if !p.rules[ruleSp]() {
						goto ok
					}
					{
						if position == len(p.Buffer) {
							goto ok
						}
						switch p.Buffer[position] {
						case ')':
							position++ // matchChar
						case '\n', '\r':
							if !p.rules[ruleNewline]() {
								goto ok
							}
						default:
							goto ok
						}
					}
					goto out
				ok:
					position = position2
				}
				if !matchDot() {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			end = position
			if !matchChar('\'') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 179 TitleDouble <- ('"' < (!('"' Sp ((&[)] ')') | (&[\n\r] Newline))) .)* > '"') */
		func() (match bool) {
			position0 := position
			if !matchChar('"') {
				goto ko
			}
			begin = position
		loop:
			{
				position1 := position
				{
					position2 := position
					if !matchChar('"') {
						goto ok
					}
					if !p.rules[ruleSp]() {
						goto ok
					}
					{
						if position == len(p.Buffer) {
							goto ok
						}
						switch p.Buffer[position] {
						case ')':
							position++ // matchChar
						case '\n', '\r':
							if !p.rules[ruleNewline]() {
								goto ok
							}
						default:
							goto ok
						}
					}
					goto out
				ok:
					position = position2
				}
				if !matchDot() {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			end = position
			if !matchChar('"') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 180 AutoLink <- (EmbeddedLink / AutoLinkUrl / AutoLinkEmail) */
		func() (match bool) {
			if !p.rules[ruleEmbeddedLink]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleAutoLinkUrl]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !p.rules[ruleAutoLinkEmail]() {
				return
			}
		ok:
			match = true
			return
		},
		/* 181 EmbeddedLink <- (StartList '`' EmbeddedRefSource '<' < [A-Za-z]+ '://' (!Newline !'>' .)+ > '>`_' '_'? {
				yy = p.mkLink(p.mkString(l.contents.str), yytext, "")
		   }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !matchChar('`') {
				goto ko
			}
			if !p.rules[ruleEmbeddedRefSource]() {
				goto ko
			}
			doarg(yySet, -2)
			if !matchChar('<') {
				goto ko
			}
			begin = position
			if !matchClass(2) {
				goto ko
			}
		loop:
			if !matchClass(2) {
				goto out
			}
			goto loop
		out:
			if !matchString("://") {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ok
			}
			goto ko
		ok:
			if peekChar('>') {
				goto ko
			}
			if !matchDot() {
				goto ko
			}
		loop3:
			{
				position1 := position
				if !p.rules[ruleNewline]() {
					goto ok6
				}
				goto out4
			ok6:
				if peekChar('>') {
					goto out4
				}
				if !matchDot() {
					goto out4
				}
				goto loop3
			out4:
				position = position1
			}
			end = position
			if !matchString(">`_") {
				goto ko
			}
			matchChar('_')
			do(89)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 182 AutoLinkUrl <- (< [A-Za-z]+ '://' (!Newline !'>' .)+ > { yy = p.mkLink(p.mkString(yytext), yytext, "") }) */
		func() (match bool) {
			position0 := position
			begin = position
			if !matchClass(2) {
				goto ko
			}
		loop:
			if !matchClass(2) {
				goto out
			}
			goto loop
		out:
			if !matchString("://") {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ok
			}
			goto ko
		ok:
			if peekChar('>') {
				goto ko
			}
			if !matchDot() {
				goto ko
			}
		loop3:
			{
				position1 := position
				if !p.rules[ruleNewline]() {
					goto ok6
				}
				goto out4
			ok6:
				if peekChar('>') {
					goto out4
				}
				if !matchDot() {
					goto out4
				}
				goto loop3
			out4:
				position = position1
			}
			end = position
			do(90)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 183 AutoLinkEmail <- ('<' 'mailto:'? < [-A-Za-z0-9+_./!%~$]+ '@' (!Newline !'>' .)+ > '>' {
		    yy = p.mkLink(p.mkString(yytext), "mailto:"+yytext, "")
		}) */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !matchString("mailto:") {
				goto ko1
			}
		ko1:
			begin = position
			if !matchClass(3) {
				goto ko
			}
		loop:
			if !matchClass(3) {
				goto out
			}
			goto loop
		out:
			if !matchChar('@') {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ok7
			}
			goto ko
		ok7:
			if peekChar('>') {
				goto ko
			}
			if !matchDot() {
				goto ko
			}
		loop5:
			{
				position1 := position
				if !p.rules[ruleNewline]() {
					goto ok8
				}
				goto out6
			ok8:
				if peekChar('>') {
					goto out6
				}
				if !matchDot() {
					goto out6
				}
				goto loop5
			out6:
				position = position1
			}
			end = position
			if !matchChar('>') {
				goto ko
			}
			do(91)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 184 Reference <- (QuotedReference / UnquotedReference) */
		func() (match bool) {
			if !p.rules[ruleQuotedReference]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleUnquotedReference]() {
				return
			}
		ok:
			match = true
			return
		},
		/* 185 QuotedReference <- (NonindentSpace '.. _`' !'``' QuotedRefSource !'``:' '`: ' RefSrc BlankLine {
		    yy = p.mkLink(p.mkString(c.contents.str), s.contents.str, "")
		    s = nil
		    c = nil
		    yy.key = REFERENCE
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			if !matchString(".. _`") {
				goto ko
			}
			if !matchString("``") {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleQuotedRefSource]() {
				goto ko
			}
			doarg(yySet, -1)
			if !matchString("``:") {
				goto ok2
			}
			goto ko
		ok2:
			if !matchString("`: ") {
				goto ko
			}
			if !p.rules[ruleRefSrc]() {
				goto ko
			}
			doarg(yySet, -2)
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			do(92)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 186 UnquotedReference <- (NonindentSpace '.. _' RefSource ': ' RefSrc BlankLine {
		    yy = p.mkLink(p.mkString(c.contents.str), s.contents.str, "")
		    s = nil
		    c = nil
		    yy.key = REFERENCE
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			if !matchString(".. _") {
				goto ko
			}
			if !p.rules[ruleRefSource]() {
				goto ko
			}
			doarg(yySet, -1)
			if !matchString(": ") {
				goto ko
			}
			if !p.rules[ruleRefSrc]() {
				goto ko
			}
			doarg(yySet, -2)
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			do(93)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 187 UrlReference <- ((NonindentSpace / Sp) ':target:' Sp RefSrc BlankLine {
		    yy = p.mkLink(p.mkString(yytext), yytext, "")
		    s = nil
		    yy.key = REFERENCE
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleNonindentSpace]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleSp]() {
				goto ko
			}
		ok:
			if !matchString(":target:") {
				goto ko
			}
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleRefSrc]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			do(94)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 188 UnquotedLinkSource <- (< (!'_' !':' !'`' Nonspacechar)+* > { yy = p.mkString(yytext) }) */
		func() (match bool) {
			begin = position
		loop:
			if position == len(p.Buffer) {
				goto out
			}
			switch p.Buffer[position] {
			case '_', ':', '`':
				goto out
			default:
				if !p.rules[ruleNonspacechar]() {
					goto out
				}
			}
		loop3:
			if position == len(p.Buffer) {
				goto out4
			}
			switch p.Buffer[position] {
			case '_', ':', '`':
				goto out4
			default:
				if !p.rules[ruleNonspacechar]() {
					goto out4
				}
			}
			goto loop3
		out4:
			goto loop
		out:
			end = position
			do(95)
			match = true
			return
		},
		/* 189 RefSource <- (< (!'_' !':' !'`' (' ' / Nonspacechar))+* > { yy = p.mkString(yytext) }) */
		func() (match bool) {
			begin = position
		loop:
			if position == len(p.Buffer) {
				goto out
			}
			switch p.Buffer[position] {
			case '_', ':', '`':
				goto out
			default:
				if !matchChar(' ') {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleNonspacechar]() {
					goto out
				}
			ok:
			}
		loop3:
			if position == len(p.Buffer) {
				goto out4
			}
			switch p.Buffer[position] {
			case '_', ':', '`':
				goto out4
			default:
				if !matchChar(' ') {
					goto nextAlt8
				}
				goto ok7
			nextAlt8:
				if !p.rules[ruleNonspacechar]() {
					goto out4
				}
			ok7:
			}
			goto loop3
		out4:
			goto loop
		out:
			end = position
			do(96)
			match = true
			return
		},
		/* 190 QuotedRefSource <- (< (!':' !'`' (' ' / Nonspacechar))+* > { yy = p.mkString(yytext) }) */
		func() (match bool) {
			begin = position
		loop:
			if position == len(p.Buffer) {
				goto out
			}
			switch p.Buffer[position] {
			case ':', '`':
				goto out
			default:
				if !matchChar(' ') {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleNonspacechar]() {
					goto out
				}
			ok:
			}
		loop3:
			if position == len(p.Buffer) {
				goto out4
			}
			switch p.Buffer[position] {
			case ':', '`':
				goto out4
			default:
				if !matchChar(' ') {
					goto nextAlt8
				}
				goto ok7
			nextAlt8:
				if !p.rules[ruleNonspacechar]() {
					goto out4
				}
			ok7:
			}
			goto loop3
		out4:
			goto loop
		out:
			end = position
			do(97)
			match = true
			return
		},
		/* 191 EmbeddedRefSource <- (< (!'<' !':' !'`' (' ' / Nonspacechar / BlankLine))+* > { yy = p.mkString(yytext) }) */
		func() (match bool) {
			begin = position
		loop:
			if position == len(p.Buffer) {
				goto out
			}
			switch p.Buffer[position] {
			case '<', ':', '`':
				goto out
			default:
				if !matchChar(' ') {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if !p.rules[ruleNonspacechar]() {
					goto nextAlt7
				}
				goto ok
			nextAlt7:
				if !p.rules[ruleBlankLine]() {
					goto out
				}
			ok:
			}
		loop3:
			if position == len(p.Buffer) {
				goto out4
			}
			switch p.Buffer[position] {
			case '<', ':', '`':
				goto out4
			default:
				if !matchChar(' ') {
					goto nextAlt9
				}
				goto ok8
			nextAlt9:
				if !p.rules[ruleNonspacechar]() {
					goto nextAlt10
				}
				goto ok8
			nextAlt10:
				if !p.rules[ruleBlankLine]() {
					goto out4
				}
			ok8:
			}
			goto loop3
		out4:
			goto loop
		out:
			end = position
			do(98)
			match = true
			return
		},
		/* 192 Label <- ('[' ((!'^' &{p.extension.Notes}) / (&. &{!p.extension.Notes})) StartList (!']' Inline { a = cons(yy, a) })* ']' {
			yy = p.mkList(LIST, a)
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !matchChar('[') {
				goto ko
			}
			if peekChar('^') {
				goto nextAlt
			}
			if !(p.extension.Notes) {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !(position < len(p.Buffer)) {
				goto ko
			}
			if !(!p.extension.Notes) {
				goto ko
			}
		ok:
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
		loop:
			{
				position1 := position
				if peekChar(']') {
					goto out
				}
				if !p.rules[ruleInline]() {
					goto out
				}
				do(99)
				goto loop
			out:
				position = position1
			}
			if !matchChar(']') {
				goto ko
			}
			do(100)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 193 RefSrc <- (< Nonspacechar+ > { yy = p.mkString(yytext)
		   yy.key = HTML }) */
		func() (match bool) {
			position0 := position
			begin = position
			if !p.rules[ruleNonspacechar]() {
				goto ko
			}
		loop:
			if !p.rules[ruleNonspacechar]() {
				goto out
			}
			goto loop
		out:
			end = position
			do(101)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 194 References <- (StartList ((Reference { a = cons(b, a) }) / SkipBlock)* { p.references = reverse(a)
		   p.state.heap.hasGlobals = true
		 } commit) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				{
					position2, thunkPosition2 := position, thunkPosition
					if !p.rules[ruleReference]() {
						goto nextAlt
					}
					doarg(yySet, -2)
					do(102)
					goto ok
				nextAlt:
					position, thunkPosition = position2, thunkPosition2
					if !p.rules[ruleSkipBlock]() {
						goto out
					}
				}
			ok:
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(103)
			if !(p.commit(thunkPosition0)) {
				goto ko
			}
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 195 Ticks2 <- ('``' !'`') */
		func() (match bool) {
			position0 := position
			if !matchString("``") {
				goto ko
			}
			if peekChar('`') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 196 Code <- (Ticks2 < ((!'`' Nonspacechar)+ / ((&[`] (!Ticks2 '`')) | (&[_] '_') | (&[\t\n\r ] (!(Sp Ticks2) ((&[\n\r] (Newline !BlankLine)) | (&[\t ] Spacechar))))))+ > Ticks2 { yy = p.mkString(yytext); yy.key = CODE }) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleTicks2]() {
				goto ko
			}
			begin = position
			if peekChar('`') {
				goto nextAlt
			}
			if !p.rules[ruleNonspacechar]() {
				goto nextAlt
			}
		loop5:
			if peekChar('`') {
				goto out6
			}
			if !p.rules[ruleNonspacechar]() {
				goto out6
			}
			goto loop5
		out6:
			goto ok
		nextAlt:
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '`':
					if !p.rules[ruleTicks2]() {
						goto ok8
					}
					goto ko
				ok8:
					if !matchChar('`') {
						goto ko
					}
				case '_':
					position++ // matchChar
				default:
					{
						position1 := position
						if !p.rules[ruleSp]() {
							goto ok9
						}
						if !p.rules[ruleTicks2]() {
							goto ok9
						}
						goto ko
					ok9:
						position = position1
					}
					{
						if position == len(p.Buffer) {
							goto ko
						}
						switch p.Buffer[position] {
						case '\n', '\r':
							if !p.rules[ruleNewline]() {
								goto ko
							}
							if !p.rules[ruleBlankLine]() {
								goto ok11
							}
							goto ko
						ok11:
							break
						case '\t', ' ':
							if !p.rules[ruleSpacechar]() {
								goto ko
							}
						default:
							goto ko
						}
					}
				}
			}
		ok:
		loop:
			{
				position1 := position
				if peekChar('`') {
					goto nextAlt13
				}
				if !p.rules[ruleNonspacechar]() {
					goto nextAlt13
				}
			loop14:
				if peekChar('`') {
					goto out15
				}
				if !p.rules[ruleNonspacechar]() {
					goto out15
				}
				goto loop14
			out15:
				goto ok12
			nextAlt13:
				{
					if position == len(p.Buffer) {
						goto out
					}
					switch p.Buffer[position] {
					case '`':
						if !p.rules[ruleTicks2]() {
							goto ok17
						}
						goto out
					ok17:
						if !matchChar('`') {
							goto out
						}
					case '_':
						position++ // matchChar
					default:
						{
							position3 := position
							if !p.rules[ruleSp]() {
								goto ok18
							}
							if !p.rules[ruleTicks2]() {
								goto ok18
							}
							goto out
						ok18:
							position = position3
						}
						{
							if position == len(p.Buffer) {
								goto out
							}
							switch p.Buffer[position] {
							case '\n', '\r':
								if !p.rules[ruleNewline]() {
									goto out
								}
								if !p.rules[ruleBlankLine]() {
									goto ok20
								}
								goto out
							ok20:
								break
							case '\t', ' ':
								if !p.rules[ruleSpacechar]() {
									goto out
								}
							default:
								goto out
							}
						}
					}
				}
			ok12:
				goto loop
			out:
				position = position1
			}
			end = position
			if !p.rules[ruleTicks2]() {
				goto ko
			}
			do(104)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 197 RawHtml <- (< (HtmlComment / HtmlBlockScript / HtmlTag) > {   if p.extension.FilterHTML {
		        yy = p.mkList(LIST, nil)
		    } else {
		        yy = p.mkString(yytext)
		        yy.key = HTML
		    }
		}) */
		func() (match bool) {
			position0 := position
			begin = position
			if !p.rules[ruleHtmlComment]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleHtmlBlockScript]() {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !p.rules[ruleHtmlTag]() {
				goto ko
			}
		ok:
			end = position
			do(105)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 198 BlankLine <- (Sp Newline) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 199 Quoted <- ((&[\'] ('\'' (!'\'' .)* '\'')) | (&[\"] ('"' (!'"' .)* '"'))) */
		func() (match bool) {
			position0 := position
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '\'':
					position++ // matchChar
				loop:
					if position == len(p.Buffer) {
						goto out
					}
					switch p.Buffer[position] {
					case '\'':
						goto out
					default:
						position++
					}
					goto loop
				out:
					if !matchChar('\'') {
						goto ko
					}
				case '"':
					position++ // matchChar
				loop4:
					if position == len(p.Buffer) {
						goto out5
					}
					switch p.Buffer[position] {
					case '"':
						goto out5
					default:
						position++
					}
					goto loop4
				out5:
					if !matchChar('"') {
						goto ko
					}
				default:
					goto ko
				}
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 200 HtmlAttribute <- (((&[\-] '-') | (&[0-9A-Za-z] [A-Za-z0-9]))+ Spnl ('=' Spnl (Quoted / (!'>' Nonspacechar)+))? Spnl) */
		func() (match bool) {
			position0 := position
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '-':
					position++ // matchChar
				default:
					if !matchClass(5) {
						goto ko
					}
				}
			}
		loop:
			{
				if position == len(p.Buffer) {
					goto out
				}
				switch p.Buffer[position] {
				case '-':
					position++ // matchChar
				default:
					if !matchClass(5) {
						goto out
					}
				}
			}
			goto loop
		out:
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			{
				position1 := position
				if !matchChar('=') {
					goto ko5
				}
				if !p.rules[ruleSpnl]() {
					goto ko5
				}
				if !p.rules[ruleQuoted]() {
					goto nextAlt
				}
				goto ok7
			nextAlt:
				if peekChar('>') {
					goto ko5
				}
				if !p.rules[ruleNonspacechar]() {
					goto ko5
				}
			loop9:
				if peekChar('>') {
					goto out10
				}
				if !p.rules[ruleNonspacechar]() {
					goto out10
				}
				goto loop9
			out10:
			ok7:
				goto ok6
			ko5:
				position = position1
			}
		ok6:
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 201 HtmlComment <- ('<!--' (!'-->' .)* '-->') */
		func() (match bool) {
			position0 := position
			if !matchString("<!--") {
				goto ko
			}
		loop:
			{
				position1 := position
				if !matchString("-->") {
					goto ok
				}
				goto out
			ok:
				if !matchDot() {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			if !matchString("-->") {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 202 HtmlTag <- ('<' Spnl '/'? [A-Za-z0-9]+ Spnl HtmlAttribute* '/'? Spnl '>') */
		func() (match bool) {
			position0 := position
			if !matchChar('<') {
				goto ko
			}
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			matchChar('/')
			if !matchClass(5) {
				goto ko
			}
		loop:
			if !matchClass(5) {
				goto out
			}
			goto loop
		out:
			if !p.rules[ruleSpnl]() {
				goto ko
			}
		loop3:
			if !p.rules[ruleHtmlAttribute]() {
				goto out4
			}
			goto loop3
		out4:
			matchChar('/')
			if !p.rules[ruleSpnl]() {
				goto ko
			}
			if !matchChar('>') {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 203 Eof <- !. */
		func() (match bool) {
			if position < len(p.Buffer) {
				return
			}
			match = true
			return
		},
		/* 204 Spacechar <- ((&[\t] '\t') | (&[ ] ' ')) */
		func() (match bool) {
			{
				if position == len(p.Buffer) {
					return
				}
				switch p.Buffer[position] {
				case '\t':
					position++ // matchChar
				case ' ':
					position++ // matchChar
				default:
					return
				}
			}
			match = true
			return
		},
		/* 205 Nonspacechar <- (!Spacechar !Newline .) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleSpacechar]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleNewline]() {
				goto ok2
			}
			goto ko
		ok2:
			if !matchDot() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 206 Newline <- ((&[\r] ('\r' '\n'?)) | (&[\n] '\n')) */
		func() (match bool) {
			position0 := position
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '\r':
					position++ // matchChar
					matchChar('\n')
				case '\n':
					position++ // matchChar
				default:
					goto ko
				}
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 207 Sp <- Spacechar* */
		func() (match bool) {
		loop:
			if !p.rules[ruleSpacechar]() {
				goto out
			}
			goto loop
		out:
			match = true
			return
		},
		/* 208 Spnl <- (Sp (Newline Sp)?) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleSp]() {
				goto ko
			}
			{
				position1 := position
				if !p.rules[ruleNewline]() {
					goto ko1
				}
				if !p.rules[ruleSp]() {
					goto ko1
				}
				goto ok
			ko1:
				position = position1
			}
		ok:
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 209 SpecialChar <- ('\'' / '"' / ((&[\\] '\\') | (&[#] '#') | (&[!] '!') | (&[<] '<') | (&[)] ')') | (&[(] '(') | (&[\]] ']') | (&[\[] '[') | (&[&] '&') | (&[`] '`') | (&[_] '_') | (&[*] '*') | (&[~] '~') | (&[\"\'\-.^] ExtendedSpecialChar))) */
		func() (match bool) {
			if !matchChar('\'') {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !matchChar('"') {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			{
				if position == len(p.Buffer) {
					return
				}
				switch p.Buffer[position] {
				case '\\':
					position++ // matchChar
				case '#':
					position++ // matchChar
				case '!':
					position++ // matchChar
				case '<':
					position++ // matchChar
				case ')':
					position++ // matchChar
				case '(':
					position++ // matchChar
				case ']':
					position++ // matchChar
				case '[':
					position++ // matchChar
				case '&':
					position++ // matchChar
				case '`':
					position++ // matchChar
				case '_':
					position++ // matchChar
				case '*':
					position++ // matchChar
				case '~':
					position++ // matchChar
				default:
					if !p.rules[ruleExtendedSpecialChar]() {
						return
					}
				}
			}
		ok:
			match = true
			return
		},
		/* 210 NormalChar <- (!((&[\n\r] Newline) | (&[\t ] Spacechar) | (&[!-#&-*\-.<\[-`~] SpecialChar)) .) */
		func() (match bool) {
			position0 := position
			{
				if position == len(p.Buffer) {
					goto ok
				}
				switch p.Buffer[position] {
				case '\n', '\r':
					if !p.rules[ruleNewline]() {
						goto ok
					}
				case '\t', ' ':
					if !p.rules[ruleSpacechar]() {
						goto ok
					}
				default:
					if !p.rules[ruleSpecialChar]() {
						goto ok
					}
				}
			}
			goto ko
		ok:
			if !matchDot() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 211 Alphanumeric <- ((&[\377] '\377') | (&[\376] '\376') | (&[\375] '\375') | (&[\374] '\374') | (&[\373] '\373') | (&[\372] '\372') | (&[\371] '\371') | (&[\370] '\370') | (&[\367] '\367') | (&[\366] '\366') | (&[\365] '\365') | (&[\364] '\364') | (&[\363] '\363') | (&[\362] '\362') | (&[\361] '\361') | (&[\360] '\360') | (&[\357] '\357') | (&[\356] '\356') | (&[\355] '\355') | (&[\354] '\354') | (&[\353] '\353') | (&[\352] '\352') | (&[\351] '\351') | (&[\350] '\350') | (&[\347] '\347') | (&[\346] '\346') | (&[\345] '\345') | (&[\344] '\344') | (&[\343] '\343') | (&[\342] '\342') | (&[\341] '\341') | (&[\340] '\340') | (&[\337] '\337') | (&[\336] '\336') | (&[\335] '\335') | (&[\334] '\334') | (&[\333] '\333') | (&[\332] '\332') | (&[\331] '\331') | (&[\330] '\330') | (&[\327] '\327') | (&[\326] '\326') | (&[\325] '\325') | (&[\324] '\324') | (&[\323] '\323') | (&[\322] '\322') | (&[\321] '\321') | (&[\320] '\320') | (&[\317] '\317') | (&[\316] '\316') | (&[\315] '\315') | (&[\314] '\314') | (&[\313] '\313') | (&[\312] '\312') | (&[\311] '\311') | (&[\310] '\310') | (&[\307] '\307') | (&[\306] '\306') | (&[\305] '\305') | (&[\304] '\304') | (&[\303] '\303') | (&[\302] '\302') | (&[\301] '\301') | (&[\300] '\300') | (&[\277] '\277') | (&[\276] '\276') | (&[\275] '\275') | (&[\274] '\274') | (&[\273] '\273') | (&[\272] '\272') | (&[\271] '\271') | (&[\270] '\270') | (&[\267] '\267') | (&[\266] '\266') | (&[\265] '\265') | (&[\264] '\264') | (&[\263] '\263') | (&[\262] '\262') | (&[\261] '\261') | (&[\260] '\260') | (&[\257] '\257') | (&[\256] '\256') | (&[\255] '\255') | (&[\254] '\254') | (&[\253] '\253') | (&[\252] '\252') | (&[\251] '\251') | (&[\250] '\250') | (&[\247] '\247') | (&[\246] '\246') | (&[\245] '\245') | (&[\244] '\244') | (&[\243] '\243') | (&[\242] '\242') | (&[\241] '\241') | (&[\240] '\240') | (&[\237] '\237') | (&[\236] '\236') | (&[\235] '\235') | (&[\234] '\234') | (&[\233] '\233') | (&[\232] '\232') | (&[\231] '\231') | (&[\230] '\230') | (&[\227] '\227') | (&[\226] '\226') | (&[\225] '\225') | (&[\224] '\224') | (&[\223] '\223') | (&[\222] '\222') | (&[\221] '\221') | (&[\220] '\220') | (&[\217] '\217') | (&[\216] '\216') | (&[\215] '\215') | (&[\214] '\214') | (&[\213] '\213') | (&[\212] '\212') | (&[\211] '\211') | (&[\210] '\210') | (&[\207] '\207') | (&[\206] '\206') | (&[\205] '\205') | (&[\204] '\204') | (&[\203] '\203') | (&[\202] '\202') | (&[\201] '\201') | (&[\200] '\200') | (&[0-9A-Za-z] [0-9A-Za-z])) */
		func() (match bool) {
			{
				if position == len(p.Buffer) {
					return
				}
				switch p.Buffer[position] {
				case '\377':
					position++ // matchChar
				case '\376':
					position++ // matchChar
				case '\375':
					position++ // matchChar
				case '\374':
					position++ // matchChar
				case '\373':
					position++ // matchChar
				case '\372':
					position++ // matchChar
				case '\371':
					position++ // matchChar
				case '\370':
					position++ // matchChar
				case '\367':
					position++ // matchChar
				case '\366':
					position++ // matchChar
				case '\365':
					position++ // matchChar
				case '\364':
					position++ // matchChar
				case '\363':
					position++ // matchChar
				case '\362':
					position++ // matchChar
				case '\361':
					position++ // matchChar
				case '\360':
					position++ // matchChar
				case '\357':
					position++ // matchChar
				case '\356':
					position++ // matchChar
				case '\355':
					position++ // matchChar
				case '\354':
					position++ // matchChar
				case '\353':
					position++ // matchChar
				case '\352':
					position++ // matchChar
				case '\351':
					position++ // matchChar
				case '\350':
					position++ // matchChar
				case '\347':
					position++ // matchChar
				case '\346':
					position++ // matchChar
				case '\345':
					position++ // matchChar
				case '\344':
					position++ // matchChar
				case '\343':
					position++ // matchChar
				case '\342':
					position++ // matchChar
				case '\341':
					position++ // matchChar
				case '\340':
					position++ // matchChar
				case '\337':
					position++ // matchChar
				case '\336':
					position++ // matchChar
				case '\335':
					position++ // matchChar
				case '\334':
					position++ // matchChar
				case '\333':
					position++ // matchChar
				case '\332':
					position++ // matchChar
				case '\331':
					position++ // matchChar
				case '\330':
					position++ // matchChar
				case '\327':
					position++ // matchChar
				case '\326':
					position++ // matchChar
				case '\325':
					position++ // matchChar
				case '\324':
					position++ // matchChar
				case '\323':
					position++ // matchChar
				case '\322':
					position++ // matchChar
				case '\321':
					position++ // matchChar
				case '\320':
					position++ // matchChar
				case '\317':
					position++ // matchChar
				case '\316':
					position++ // matchChar
				case '\315':
					position++ // matchChar
				case '\314':
					position++ // matchChar
				case '\313':
					position++ // matchChar
				case '\312':
					position++ // matchChar
				case '\311':
					position++ // matchChar
				case '\310':
					position++ // matchChar
				case '\307':
					position++ // matchChar
				case '\306':
					position++ // matchChar
				case '\305':
					position++ // matchChar
				case '\304':
					position++ // matchChar
				case '\303':
					position++ // matchChar
				case '\302':
					position++ // matchChar
				case '\301':
					position++ // matchChar
				case '\300':
					position++ // matchChar
				case '\277':
					position++ // matchChar
				case '\276':
					position++ // matchChar
				case '\275':
					position++ // matchChar
				case '\274':
					position++ // matchChar
				case '\273':
					position++ // matchChar
				case '\272':
					position++ // matchChar
				case '\271':
					position++ // matchChar
				case '\270':
					position++ // matchChar
				case '\267':
					position++ // matchChar
				case '\266':
					position++ // matchChar
				case '\265':
					position++ // matchChar
				case '\264':
					position++ // matchChar
				case '\263':
					position++ // matchChar
				case '\262':
					position++ // matchChar
				case '\261':
					position++ // matchChar
				case '\260':
					position++ // matchChar
				case '\257':
					position++ // matchChar
				case '\256':
					position++ // matchChar
				case '\255':
					position++ // matchChar
				case '\254':
					position++ // matchChar
				case '\253':
					position++ // matchChar
				case '\252':
					position++ // matchChar
				case '\251':
					position++ // matchChar
				case '\250':
					position++ // matchChar
				case '\247':
					position++ // matchChar
				case '\246':
					position++ // matchChar
				case '\245':
					position++ // matchChar
				case '\244':
					position++ // matchChar
				case '\243':
					position++ // matchChar
				case '\242':
					position++ // matchChar
				case '\241':
					position++ // matchChar
				case '\240':
					position++ // matchChar
				case '\237':
					position++ // matchChar
				case '\236':
					position++ // matchChar
				case '\235':
					position++ // matchChar
				case '\234':
					position++ // matchChar
				case '\233':
					position++ // matchChar
				case '\232':
					position++ // matchChar
				case '\231':
					position++ // matchChar
				case '\230':
					position++ // matchChar
				case '\227':
					position++ // matchChar
				case '\226':
					position++ // matchChar
				case '\225':
					position++ // matchChar
				case '\224':
					position++ // matchChar
				case '\223':
					position++ // matchChar
				case '\222':
					position++ // matchChar
				case '\221':
					position++ // matchChar
				case '\220':
					position++ // matchChar
				case '\217':
					position++ // matchChar
				case '\216':
					position++ // matchChar
				case '\215':
					position++ // matchChar
				case '\214':
					position++ // matchChar
				case '\213':
					position++ // matchChar
				case '\212':
					position++ // matchChar
				case '\211':
					position++ // matchChar
				case '\210':
					position++ // matchChar
				case '\207':
					position++ // matchChar
				case '\206':
					position++ // matchChar
				case '\205':
					position++ // matchChar
				case '\204':
					position++ // matchChar
				case '\203':
					position++ // matchChar
				case '\202':
					position++ // matchChar
				case '\201':
					position++ // matchChar
				case '\200':
					position++ // matchChar
				default:
					if !matchClass(4) {
						return
					}
				}
			}
			match = true
			return
		},
		/* 212 AlphanumericAscii <- [A-Za-z0-9] */
		func() (match bool) {
			if !matchClass(5) {
				return
			}
			match = true
			return
		},
		/* 213 Digit <- [0-9] */
		func() (match bool) {
			if !matchClass(0) {
				return
			}
			match = true
			return
		},
		/* 214 HexEntity <- (< '&' '#' [Xx] [0-9a-fA-F]+ ';' >) */
		func() (match bool) {
			position0 := position
			begin = position
			if !matchChar('&') {
				goto ko
			}
			if !matchChar('#') {
				goto ko
			}
			if !matchClass(6) {
				goto ko
			}
			if !matchClass(7) {
				goto ko
			}
		loop:
			if !matchClass(7) {
				goto out
			}
			goto loop
		out:
			if !matchChar(';') {
				goto ko
			}
			end = position
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 215 DecEntity <- (< '&' '#' [0-9]+ > ';' >) */
		func() (match bool) {
			position0 := position
			begin = position
			if !matchChar('&') {
				goto ko
			}
			if !matchChar('#') {
				goto ko
			}
			if !matchClass(0) {
				goto ko
			}
		loop:
			if !matchClass(0) {
				goto out
			}
			goto loop
		out:
			end = position
			if !matchChar(';') {
				goto ko
			}
			end = position
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 216 CharEntity <- (< '&' [A-Za-z0-9]+ ';' >) */
		func() (match bool) {
			position0 := position
			begin = position
			if !matchChar('&') {
				goto ko
			}
			if !matchClass(5) {
				goto ko
			}
		loop:
			if !matchClass(5) {
				goto out
			}
			goto loop
		out:
			if !matchChar(';') {
				goto ko
			}
			end = position
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 217 NonindentSpace <- ('   ' / '  ' / ' ' / '') */
		func() (match bool) {
			if !matchString("   ") {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !matchString("  ") {
				goto nextAlt3
			}
			goto ok
		nextAlt3:
			if !matchChar(' ') {
				goto nextAlt4
			}
			goto ok
		nextAlt4:
		ok:
			match = true
			return
		},
		/* 218 Indent <- ((&[ ] '    ') | (&[\t] '\t')) */
		func() (match bool) {
			{
				if position == len(p.Buffer) {
					return
				}
				switch p.Buffer[position] {
				case ' ':
					position++
					if !matchString("   ") {
						return
					}
				case '\t':
					position++ // matchChar
				default:
					return
				}
			}
			match = true
			return
		},
		/* 219 IndentedLine <- (Indent Line) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleIndent]() {
				goto ko
			}
			if !p.rules[ruleLine]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 220 OptionallyIndentedLine <- (Indent? Line) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleIndent]() {
				goto ko1
			}
		ko1:
			if !p.rules[ruleLine]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 221 StartList <- (&. { yy = nil }) */
		func() (match bool) {
			if !(position < len(p.Buffer)) {
				return
			}
			do(106)
			match = true
			return
		},
		/* 222 DoctestLine <- ('>>> ' RawLine { yy = p.mkString(">>> " + yytext) }) */
		func() (match bool) {
			position0 := position
			if !matchString(">>> ") {
				goto ko
			}
			if !p.rules[ruleRawLine]() {
				goto ko
			}
			do(107)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 223 Line <- (RawLine { yy = p.mkString(yytext) }) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleRawLine]() {
				goto ko
			}
			do(108)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 224 RawLine <- ((< (!'\r' !'\n' .)* Newline >) / (< .+ > !.)) */
		func() (match bool) {
			position0 := position
			{
				position1 := position
				begin = position
			loop:
				if position == len(p.Buffer) {
					goto out
				}
				switch p.Buffer[position] {
				case '\r', '\n':
					goto out
				default:
					position++
				}
				goto loop
			out:
				if !p.rules[ruleNewline]() {
					goto nextAlt
				}
				end = position
				goto ok
			nextAlt:
				position = position1
				begin = position
				if !matchDot() {
					goto ko
				}
			loop5:
				if !matchDot() {
					goto out6
				}
				goto loop5
			out6:
				end = position
				if position < len(p.Buffer) {
					goto ko
				}
			}
		ok:
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 225 SkipBlock <- (HtmlBlock / ((!'#' !SetextBottom !BlankLine RawLine)+ BlankLine*) / BlankLine+ / RawLine) */
		func() (match bool) {
			position0 := position
			{
				position1 := position
				if !p.rules[ruleHtmlBlock]() {
					goto nextAlt
				}
				goto ok
			nextAlt:
				if peekChar('#') {
					goto nextAlt3
				}
				if !p.rules[ruleSetextBottom]() {
					goto ok6
				}
				goto nextAlt3
			ok6:
				if !p.rules[ruleBlankLine]() {
					goto ok7
				}
				goto nextAlt3
			ok7:
				if !p.rules[ruleRawLine]() {
					goto nextAlt3
				}
			loop:
				{
					position2 := position
					if peekChar('#') {
						goto out
					}
					if !p.rules[ruleSetextBottom]() {
						goto ok8
					}
					goto out
				ok8:
					if !p.rules[ruleBlankLine]() {
						goto ok9
					}
					goto out
				ok9:
					if !p.rules[ruleRawLine]() {
						goto out
					}
					goto loop
				out:
					position = position2
				}
			loop10:
				if !p.rules[ruleBlankLine]() {
					goto out11
				}
				goto loop10
			out11:
				goto ok
			nextAlt3:
				position = position1
				if !p.rules[ruleBlankLine]() {
					goto nextAlt12
				}
			loop13:
				if !p.rules[ruleBlankLine]() {
					goto out14
				}
				goto loop13
			out14:
				goto ok
			nextAlt12:
				position = position1
				if !p.rules[ruleRawLine]() {
					goto ko
				}
			}
		ok:
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 226 ExtendedSpecialChar <- ((&[^] (&{p.extension.Notes} '^')) | (&[\"\'\-.] (&{p.extension.Smart} ((&[\"] '"') | (&[\'] '\'') | (&[\-] '-') | (&[.] '.'))))) */
		func() (match bool) {
			position0 := position
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '^':
					if !(p.extension.Notes) {
						goto ko
					}
					if !matchChar('^') {
						goto ko
					}
				default:
					if !(p.extension.Smart) {
						goto ko
					}
					{
						if position == len(p.Buffer) {
							goto ko
						}
						switch p.Buffer[position] {
						case '"':
							position++ // matchChar
						case '\'':
							position++ // matchChar
						case '-':
							position++ // matchChar
						case '.':
							position++ // matchChar
						default:
							goto ko
						}
					}
				}
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 227 Smart <- (&{p.extension.Smart} (SingleQuoted / ((&[\'] Apostrophe) | (&[\"] DoubleQuoted) | (&[\-] Dash) | (&[.] Ellipsis)))) */
		func() (match bool) {
			if !(p.extension.Smart) {
				return
			}
			if !p.rules[ruleSingleQuoted]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			{
				if position == len(p.Buffer) {
					return
				}
				switch p.Buffer[position] {
				case '\'':
					if !p.rules[ruleApostrophe]() {
						return
					}
				case '"':
					if !p.rules[ruleDoubleQuoted]() {
						return
					}
				case '-':
					if !p.rules[ruleDash]() {
						return
					}
				case '.':
					if !p.rules[ruleEllipsis]() {
						return
					}
				default:
					return
				}
			}
		ok:
			match = true
			return
		},
		/* 228 Apostrophe <- ('\'' { yy = p.mkElem(APOSTROPHE) }) */
		func() (match bool) {
			position0 := position
			if !matchChar('\'') {
				goto ko
			}
			do(109)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 229 Ellipsis <- (('...' / '. . .') { yy = p.mkElem(ELLIPSIS) }) */
		func() (match bool) {
			position0 := position
			if !matchString("...") {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !matchString(". . .") {
				goto ko
			}
		ok:
			do(110)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 230 Dash <- (EmDash / EnDash) */
		func() (match bool) {
			if !p.rules[ruleEmDash]() {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !p.rules[ruleEnDash]() {
				return
			}
		ok:
			match = true
			return
		},
		/* 231 EnDash <- ('-' &[0-9] { yy = p.mkElem(ENDASH) }) */
		func() (match bool) {
			position0 := position
			if !matchChar('-') {
				goto ko
			}
			if !peekClass(0) {
				goto ko
			}
			do(111)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 232 EmDash <- (('---' / '--') { yy = p.mkElem(EMDASH) }) */
		func() (match bool) {
			position0 := position
			if !matchString("---") {
				goto nextAlt
			}
			goto ok
		nextAlt:
			if !matchString("--") {
				goto ko
			}
		ok:
			do(112)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 233 SingleQuoteStart <- ('\'' !((&[\n\r] Newline) | (&[\t ] Spacechar))) */
		func() (match bool) {
			position0 := position
			if !matchChar('\'') {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ok
				}
				switch p.Buffer[position] {
				case '\n', '\r':
					if !p.rules[ruleNewline]() {
						goto ok
					}
				case '\t', ' ':
					if !p.rules[ruleSpacechar]() {
						goto ok
					}
				default:
					goto ok
				}
			}
			goto ko
		ok:
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 234 SingleQuoteEnd <- ('\'' !Alphanumeric) */
		func() (match bool) {
			position0 := position
			if !matchChar('\'') {
				goto ko
			}
			if !p.rules[ruleAlphanumeric]() {
				goto ok
			}
			goto ko
		ok:
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 235 SingleQuoted <- (SingleQuoteStart StartList (!SingleQuoteEnd Inline { a = cons(b, a) })+ SingleQuoteEnd { yy = p.mkList(SINGLEQUOTED, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !p.rules[ruleSingleQuoteStart]() {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleSingleQuoteEnd]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleInline]() {
				goto ko
			}
			doarg(yySet, -2)
			do(113)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleSingleQuoteEnd]() {
					goto ok4
				}
				goto out
			ok4:
				if !p.rules[ruleInline]() {
					goto out
				}
				doarg(yySet, -2)
				do(113)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			if !p.rules[ruleSingleQuoteEnd]() {
				goto ko
			}
			do(114)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 236 DoubleQuoteStart <- '"' */
		func() (match bool) {
			if !matchChar('"') {
				return
			}
			match = true
			return
		},
		/* 237 DoubleQuoteEnd <- '"' */
		func() (match bool) {
			if !matchChar('"') {
				return
			}
			match = true
			return
		},
		/* 238 DoubleQuoted <- ('"' StartList (!'"' Inline { a = cons(b, a) })+ '"' { yy = p.mkList(DOUBLEQUOTED, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !matchChar('"') {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if peekChar('"') {
				goto ko
			}
			if !p.rules[ruleInline]() {
				goto ko
			}
			doarg(yySet, -2)
			do(115)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if peekChar('"') {
					goto out
				}
				if !p.rules[ruleInline]() {
					goto out
				}
				doarg(yySet, -2)
				do(115)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			if !matchChar('"') {
				goto ko
			}
			do(116)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 239 NoteReference <- (&{p.extension.Notes} RawNoteReference {
		    p.state.heap.hasGlobals = true
		    if match, ok := p.find_note(ref.contents.str); ok {
		        yy = p.mkElem(NOTE)
		        yy.children = match.children
		        yy.contents.str = ""
		    } else {
		        yy = p.mkString("[^"+ref.contents.str+"]")
		    }
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !(p.extension.Notes) {
				goto ko
			}
			if !p.rules[ruleRawNoteReference]() {
				goto ko
			}
			doarg(yySet, -1)
			do(117)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 240 RawNoteReference <- ('[^' < (!Newline !']' .)+ > ']' { yy = p.mkString(yytext) }) */
		func() (match bool) {
			position0 := position
			if !matchString("[^") {
				goto ko
			}
			begin = position
			if !p.rules[ruleNewline]() {
				goto ok
			}
			goto ko
		ok:
			if peekChar(']') {
				goto ko
			}
			if !matchDot() {
				goto ko
			}
		loop:
			{
				position1 := position
				if !p.rules[ruleNewline]() {
					goto ok4
				}
				goto out
			ok4:
				if peekChar(']') {
					goto out
				}
				if !matchDot() {
					goto out
				}
				goto loop
			out:
				position = position1
			}
			end = position
			if !matchChar(']') {
				goto ko
			}
			do(118)
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 241 Note <- (&{p.extension.Notes} NonindentSpace RawNoteReference ':' Sp StartList (RawNoteBlock { a = cons(yy, a) }) (&Indent RawNoteBlock { a = cons(yy, a) })* {   yy = p.mkList(NOTE, a)
		    yy.contents.str = ref.contents.str
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !(p.extension.Notes) {
				goto ko
			}
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			if !p.rules[ruleRawNoteReference]() {
				goto ko
			}
			doarg(yySet, -1)
			if !matchChar(':') {
				goto ko
			}
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -2)
			if !p.rules[ruleRawNoteBlock]() {
				goto ko
			}
			do(119)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				{
					position2 := position
					if !p.rules[ruleIndent]() {
						goto out
					}
					position = position2
				}
				if !p.rules[ruleRawNoteBlock]() {
					goto out
				}
				do(120)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(121)
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 242 Footnote <- ('[#' StartList (!']' Inline { a = cons(yy, a) })+ ']_' {
			yy = p.mkList(NOTE, a)
			//p.state.heap.hasGlobals = true
			yy.contents.str = ""
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !matchString("[#") {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if peekChar(']') {
				goto ko
			}
			if !p.rules[ruleInline]() {
				goto ko
			}
			do(122)
		loop:
			{
				position1 := position
				if peekChar(']') {
					goto out
				}
				if !p.rules[ruleInline]() {
					goto out
				}
				do(122)
				goto loop
			out:
				position = position1
			}
			if !matchString("]_") {
				goto ko
			}
			do(123)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 243 Notes <- (StartList ((Note { a = cons(b, a) }) / SkipBlock)* { p.notes = reverse(a) } commit) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 2)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				{
					position2, thunkPosition2 := position, thunkPosition
					if !p.rules[ruleNote]() {
						goto nextAlt
					}
					doarg(yySet, -2)
					do(124)
					goto ok
				nextAlt:
					position, thunkPosition = position2, thunkPosition2
					if !p.rules[ruleSkipBlock]() {
						goto out
					}
				}
			ok:
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(125)
			if !(p.commit(thunkPosition0)) {
				goto ko
			}
			doarg(yyPop, 2)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 244 RawNoteBlock <- (StartList (!BlankLine OptionallyIndentedLine { a = cons(yy, a) })+ (< BlankLine* > { a = cons(p.mkString(yytext), a) }) {   yy = p.mkStringFromList(a, true)
		       p.state.heap.hasGlobals = true
		   yy.key = RAW
		 }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleBlankLine]() {
				goto ok
			}
			goto ko
		ok:
			if !p.rules[ruleOptionallyIndentedLine]() {
				goto ko
			}
			do(126)
		loop:
			{
				position1 := position
				if !p.rules[ruleBlankLine]() {
					goto ok4
				}
				goto out
			ok4:
				if !p.rules[ruleOptionallyIndentedLine]() {
					goto out
				}
				do(126)
				goto loop
			out:
				position = position1
			}
			begin = position
		loop5:
			if !p.rules[ruleBlankLine]() {
				goto out6
			}
			goto loop5
		out6:
			end = position
			do(127)
			do(128)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 245 DefinitionList <- (&{p.extension.Dlists} StartList (Definition { a = cons(yy, a) })+ { yy = p.mkList(DEFINITIONLIST, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !(p.extension.Dlists) {
				goto ko
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleDefinition]() {
				goto ko
			}
			do(129)
		loop:
			{
				position1, thunkPosition1 := position, thunkPosition
				if !p.rules[ruleDefinition]() {
					goto out
				}
				do(129)
				goto loop
			out:
				position, thunkPosition = position1, thunkPosition1
			}
			do(130)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 246 Definition <- (&(NonindentSpace !Defmark Nonspacechar RawLine BlankLine? Defmark) StartList (DListTitle { a = cons(yy, a) })+ (DefTight / DefLoose) {
			for e := yy.children; e != nil; e = e.next {
				e.key = DEFDATA
			}
			a = cons(yy, a)
		} { yy = p.mkList(LIST, a) }) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			{
				position1 := position
				if !p.rules[ruleNonindentSpace]() {
					goto ko
				}
				if !p.rules[ruleDefmark]() {
					goto ok
				}
				goto ko
			ok:
				if !p.rules[ruleNonspacechar]() {
					goto ko
				}
				if !p.rules[ruleRawLine]() {
					goto ko
				}
				if !p.rules[ruleBlankLine]() {
					goto ko3
				}
			ko3:
				if !p.rules[ruleDefmark]() {
					goto ko
				}
				position = position1
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleDListTitle]() {
				goto ko
			}
			do(131)
		loop:
			{
				position2, thunkPosition2 := position, thunkPosition
				if !p.rules[ruleDListTitle]() {
					goto out
				}
				do(131)
				goto loop
			out:
				position, thunkPosition = position2, thunkPosition2
			}
			if !p.rules[ruleDefTight]() {
				goto nextAlt
			}
			goto ok7
		nextAlt:
			if !p.rules[ruleDefLoose]() {
				goto ko
			}
		ok7:
			do(132)
			do(133)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 247 DListTitle <- (NonindentSpace !Defmark &Nonspacechar StartList (!Endline Inline { a = cons(yy, a) })+ Sp Newline {	yy = p.mkList(LIST, a)
			yy.key = DEFTITLE
		}) */
		func() (match bool) {
			position0, thunkPosition0 := position, thunkPosition
			doarg(yyPush, 1)
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			if !p.rules[ruleDefmark]() {
				goto ok
			}
			goto ko
		ok:
			{
				position1 := position
				if !p.rules[ruleNonspacechar]() {
					goto ko
				}
				position = position1
			}
			if !p.rules[ruleStartList]() {
				goto ko
			}
			doarg(yySet, -1)
			if !p.rules[ruleEndline]() {
				goto ok5
			}
			goto ko
		ok5:
			if !p.rules[ruleInline]() {
				goto ko
			}
			do(134)
		loop:
			{
				position2 := position
				if !p.rules[ruleEndline]() {
					goto ok6
				}
				goto out
			ok6:
				if !p.rules[ruleInline]() {
					goto out
				}
				do(134)
				goto loop
			out:
				position = position2
			}
			if !p.rules[ruleSp]() {
				goto ko
			}
			if !p.rules[ruleNewline]() {
				goto ko
			}
			do(135)
			doarg(yyPop, 1)
			match = true
			return
		ko:
			position, thunkPosition = position0, thunkPosition0
			return
		},
		/* 248 DefTight <- (&Defmark ListTight) */
		func() (match bool) {
			{
				position1 := position
				if !p.rules[ruleDefmark]() {
					return
				}
				position = position1
			}
			if !p.rules[ruleListTight]() {
				return
			}
			match = true
			return
		},
		/* 249 DefLoose <- (BlankLine &Defmark ListLoose) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleBlankLine]() {
				goto ko
			}
			{
				position1 := position
				if !p.rules[ruleDefmark]() {
					goto ko
				}
				position = position1
			}
			if !p.rules[ruleListLoose]() {
				goto ko
			}
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 250 Defmark <- (NonindentSpace ((&[~] '~') | (&[:] ':')) Spacechar+) */
		func() (match bool) {
			position0 := position
			if !p.rules[ruleNonindentSpace]() {
				goto ko
			}
			{
				if position == len(p.Buffer) {
					goto ko
				}
				switch p.Buffer[position] {
				case '~':
					position++ // matchChar
				case ':':
					position++ // matchChar
				default:
					goto ko
				}
			}
			if !p.rules[ruleSpacechar]() {
				goto ko
			}
		loop:
			if !p.rules[ruleSpacechar]() {
				goto out
			}
			goto loop
		out:
			match = true
			return
		ko:
			position = position0
			return
		},
		/* 251 DefMarker <- (&{p.extension.Dlists} Defmark) */
		func() (match bool) {
			if !(p.extension.Dlists) {
				return
			}
			if !p.rules[ruleDefmark]() {
				return
			}
			match = true
			return
		},
	}
}

/*
 * List manipulation functions
 */

/* cons - cons an element onto a list, returning pointer to new head
 */
func cons(new, list *element) *element {
	new.next = list
	return new
}

/* reverse - reverse a list, returning pointer to new list
 */
func reverse(list *element) (new *element) {
	for list != nil {
		next := list.next
		new = cons(list, new)
		list = next
	}
	return
}

/*
 *  Auxiliary functions for parsing actions.
 *  These make it easier to build up data structures (including lists)
 *  in the parsing actions.
 */

/* p.mkElem - generic constructor for element
 */
func (p *yyParser) mkElem(key int) *element {
	r := p.state.heap.row
	if len(r) == 0 {
		r = p.state.heap.nextRow()
	}
	e := &r[0]
	*e = element{}
	p.state.heap.row = r[1:]
	e.key = key
	return e
}

/* p.mkString - constructor for STR element
 */
func (p *yyParser) mkString(s string) (result *element) {
	result = p.mkElem(STR)
	result.contents.str = s
	return
}

/* p.mkStringFromList - makes STR element by concatenating a
 * reversed list of strings, adding optional extra newline
 */
func (p *yyParser) mkStringFromList(list *element, extra_newline bool) (result *element) {
	s := ""
	for list = reverse(list); list != nil; list = list.next {
		s += list.contents.str
	}

	if extra_newline {
		s += "\n"
	}
	result = p.mkElem(STR)
	result.contents.str = s
	return
}

/* p.mkList - makes new list with key 'key' and children the reverse of 'lst'.
 * This is designed to be used with cons to build lists in a parser action.
 * The reversing is necessary because cons adds to the head of a list.
 */
func (p *yyParser) mkList(key int, lst *element) (el *element) {
	el = p.mkElem(key)
	el.children = reverse(lst)
	return
}

/* p.mkLink - constructor for LINK element
 */
func (p *yyParser) mkLink(label *element, url, title string) (el *element) {
	el = p.mkElem(LINK)
	el.contents.link = &link{label: label, url: url, title: title}
	return
}

func (p *yyParser) mkCodeBlock(list *element, lang string) (result *element) {
	s := ""
	for list = reverse(list); list != nil; list = list.next {
		s += list.contents.str
	}

	result = p.mkElem(CODEBLOCK)
	result.contents.str = s
	result.contents.code = &code{lang: lang}
	return
}

/* match_inlines - returns true if inline lists match (case-insensitive...)
 */
func match_inlines(l1, l2 *element) bool {
	for l1 != nil && l2 != nil {
		if l1.key != l2.key {
			return false
		}
		switch l1.key {
		case SPACE, LINEBREAK, ELLIPSIS, EMDASH, ENDASH, APOSTROPHE:
			break
		case CODE, STR, HTML:
			if strings.ToUpper(l1.contents.str) != strings.ToUpper(l2.contents.str) {
				return false
			}
		case EMPH, STRONG, LIST, SINGLEQUOTED, DOUBLEQUOTED:
			if !match_inlines(l1.children, l2.children) {
				return false
			}
		case LINK, IMAGE:
			return false /* No links or images within links */
		default:
			log.Fatalf("match_inlines encountered unknown key = %d\n", l1.key)
		}
		l1 = l1.next
		l2 = l2.next
	}
	return l1 == nil && l2 == nil /* return true if both lists exhausted */
}

/* find_reference - return true if link found in references matching label.
 * 'link' is modified with the matching url and title.
 */
func (p *yyParser) findReference(label *element) (*link, bool) {
	for cur := p.references; cur != nil; cur = cur.next {
		l := cur.contents.link
		if match_inlines(label, l.label) {
			return l, true
		}
	}
	return nil, false
}

/* find_note - return true if note found in notes matching label.
 * if found, 'result' is set to point to matched note.
 */
func (p *yyParser) find_note(label string) (*element, bool) {
	for el := p.notes; el != nil; el = el.next {
		if label == el.contents.str {
			return el, true
		}
	}
	return nil, false
}

/* print tree of elements, for debugging only.
 */
func print_tree(w io.Writer, elt *element, indent int) {
	var key string

	for elt != nil {
		for i := 0; i < indent; i++ {
			fmt.Fprint(w, "\t")
		}
		key = keynames[elt.key]
		if key == "" {
			key = "?"
		}
		if elt.key == STR {
			fmt.Fprintf(w, "%p:\t%s\t'%s'\n", elt, key, elt.contents.str)
		} else {
			fmt.Fprintf(w, "%p:\t%s %p\n", elt, key, elt.next)
		}
		if elt.children != nil {
			print_tree(w, elt.children, indent+1)
		}
		elt = elt.next
	}
}

var HeadingLevel int = 0
var HeadingKeys [6]string /* 0: H1, 5: H6 */
func getHeadingElm(label string) int {
	retLevel := -1
	for level, val := range HeadingKeys {
		if label == val {
			retLevel = level
			break
		}
	}
	if retLevel == -1 {
		HeadingKeys[HeadingLevel] = label
		retLevel = HeadingLevel
		HeadingLevel += 1
	}
	switch retLevel {
	case 0:
		return H1
	case 1:
		return H2
	case 2:
		return H3
	case 3:
		return H4
	case 4:
		return H5
	case 5:
		return H6
	}
	panic("not support heading level")
}

func initParser() {
	HeadingLevel = 0
	HeadingKeys = [6]string{}
}

var keynames = [numVAL]string{
	LIST:           "LIST",
	RAW:            "RAW",
	SPACE:          "SPACE",
	LINEBREAK:      "LINEBREAK",
	ELLIPSIS:       "ELLIPSIS",
	EMDASH:         "EMDASH",
	ENDASH:         "ENDASH",
	APOSTROPHE:     "APOSTROPHE",
	SINGLEQUOTED:   "SINGLEQUOTED",
	DOUBLEQUOTED:   "DOUBLEQUOTED",
	STR:            "STR",
	LINK:           "LINK",
	IMAGE:          "IMAGE",
	CODE:           "CODE",
	HTML:           "HTML",
	EMPH:           "EMPH",
	STRONG:         "STRONG",
	STRIKE:         "STRIKE",
	PLAIN:          "PLAIN",
	PARA:           "PARA",
	LISTITEM:       "LISTITEM",
	BULLETLIST:     "BULLETLIST",
	ORDEREDLIST:    "ORDEREDLIST",
	H1TITLE:        "H1TITLE",
	H1:             "H1",
	H2:             "H2",
	H3:             "H3",
	H4:             "H4",
	H5:             "H5",
	H6:             "H6",
	TABLE:          "TABLE",
	TABLESEPARATOR: "TABLESEPARATOR",
	TABLECELL:      "TABLECELL",
	CELLSPAN:       "CELLSPAN",
	TABLEROW:       "TABLEROW",
	TABLEBODY:      "TABLEBODY",
	TABLEHEAD:      "TABLEHEAD",
	CODEBLOCK:      "CODEBLOCK",
	BLOCKQUOTE:     "BLOCKQUOTE",
	VERBATIM:       "VERBATIM",
	HTMLBLOCK:      "HTMLBLOCK",
	HRULE:          "HRULE",
	REFERENCE:      "REFERENCE",
	NOTE:           "NOTE",
	DEFINITIONLIST: "DEFINITIONLIST",
	DEFTITLE:       "DEFTITLE",
	DEFDATA:        "DEFDATA",
}
