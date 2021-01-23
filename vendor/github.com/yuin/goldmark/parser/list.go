package parser

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"strconv"
)

type listItemType int

const (
	notList listItemType = iota
	bulletList
	orderedList
)

// Same as
// `^(([ ]*)([\-\*\+]))(\s+.*)?\n?$`.FindSubmatchIndex or
// `^(([ ]*)(\d{1,9}[\.\)]))(\s+.*)?\n?$`.FindSubmatchIndex
func parseListItem(line []byte) ([6]int, listItemType) {
	i := 0
	l := len(line)
	ret := [6]int{}
	for ; i < l && line[i] == ' '; i++ {
		c := line[i]
		if c == '\t' {
			return ret, notList
		}
	}
	if i > 3 {
		return ret, notList
	}
	ret[0] = 0
	ret[1] = i
	ret[2] = i
	var typ listItemType
	if i < l && (line[i] == '-' || line[i] == '*' || line[i] == '+') {
		i++
		ret[3] = i
		typ = bulletList
	} else if i < l {
		for ; i < l && util.IsNumeric(line[i]); i++ {
		}
		ret[3] = i
		if ret[3] == ret[2] || ret[3]-ret[2] > 9 {
			return ret, notList
		}
		if i < l && (line[i] == '.' || line[i] == ')') {
			i++
			ret[3] = i
		} else {
			return ret, notList
		}
		typ = orderedList
	} else {
		return ret, notList
	}
	if i < l && line[i] != '\n' {
		w, _ := util.IndentWidth(line[i:], 0)
		if w == 0 {
			return ret, notList
		}
	}
	if i >= l {
		ret[4] = -1
		ret[5] = -1
		return ret, typ
	}
	ret[4] = i
	ret[5] = len(line)
	if line[ret[5]-1] == '\n' && line[i] != '\n' {
		ret[5]--
	}
	return ret, typ
}

func matchesListItem(source []byte, strict bool) ([6]int, listItemType) {
	m, typ := parseListItem(source)
	if typ != notList && (!strict || strict && m[1] < 4) {
		return m, typ
	}
	return m, notList
}

func calcListOffset(source []byte, match [6]int) int {
	offset := 0
	if match[4] < 0 || util.IsBlank(source[match[4]:]) { // list item starts with a blank line
		offset = 1
	} else {
		offset, _ = util.IndentWidth(source[match[4]:], match[4])
		if offset > 4 { // offseted codeblock
			offset = 1
		}
	}
	return offset
}

func lastOffset(node ast.Node) int {
	lastChild := node.LastChild()
	if lastChild != nil {
		return lastChild.(*ast.ListItem).Offset
	}
	return 0
}

type listParser struct {
}

var defaultListParser = &listParser{}

// NewListParser returns a new BlockParser that
// parses lists.
// This parser must take precedence over the ListItemParser.
func NewListParser() BlockParser {
	return defaultListParser
}

func (b *listParser) Trigger() []byte {
	return []byte{'-', '+', '*', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
}

func (b *listParser) Open(parent ast.Node, reader text.Reader, pc Context) (ast.Node, State) {
	last := pc.LastOpenedBlock().Node
	if _, lok := last.(*ast.List); lok || pc.Get(skipListParser) != nil {
		pc.Set(skipListParser, nil)
		return nil, NoChildren
	}
	line, _ := reader.PeekLine()
	match, typ := matchesListItem(line, true)
	if typ == notList {
		return nil, NoChildren
	}
	start := -1
	if typ == orderedList {
		number := line[match[2] : match[3]-1]
		start, _ = strconv.Atoi(string(number))
	}

	if ast.IsParagraph(last) && last.Parent() == parent {
		// we allow only lists starting with 1 to interrupt paragraphs.
		if typ == orderedList && start != 1 {
			return nil, NoChildren
		}
		//an empty list item cannot interrupt a paragraph:
		if match[5]-match[4] == 1 {
			return nil, NoChildren
		}
	}

	marker := line[match[3]-1]
	node := ast.NewList(marker)
	if start > -1 {
		node.Start = start
	}
	return node, HasChildren
}

func (b *listParser) Continue(node ast.Node, reader text.Reader, pc Context) State {
	list := node.(*ast.List)
	line, _ := reader.PeekLine()
	if util.IsBlank(line) {
		// A list item can begin with at most one blank line
		if node.ChildCount() == 1 && node.LastChild().ChildCount() == 0 {
			return Close
		}
		return Continue | HasChildren
	}

	// "offset" means a width that bar indicates.
	//    -  aaaaaaaa
	// |----|
	//
	// If the indent is less than the last offset like
	// - a
	//  - b          <--- current line
	// it maybe a new child of the list.
	offset := lastOffset(node)
	indent, _ := util.IndentWidth(line, reader.LineOffset())

	if indent < offset {
		if indent < 4 {
			match, typ := matchesListItem(line, false) // may have a leading spaces more than 3
			if typ != notList && match[1]-offset < 4 {
				marker := line[match[3]-1]
				if !list.CanContinue(marker, typ == orderedList) {
					return Close
				}
				// Thematic Breaks take precedence over lists
				if isThematicBreak(line[match[3]-1:], 0) {
					isHeading := false
					last := pc.LastOpenedBlock().Node
					if ast.IsParagraph(last) {
						c, ok := matchesSetextHeadingBar(line[match[3]-1:])
						if ok && c == '-' {
							isHeading = true
						}
					}
					if !isHeading {
						return Close
					}
				}

				return Continue | HasChildren
			}
		}
		return Close
	}
	return Continue | HasChildren
}

func (b *listParser) Close(node ast.Node, reader text.Reader, pc Context) {
	list := node.(*ast.List)

	for c := node.FirstChild(); c != nil && list.IsTight; c = c.NextSibling() {
		if c.FirstChild() != nil && c.FirstChild() != c.LastChild() {
			for c1 := c.FirstChild().NextSibling(); c1 != nil; c1 = c1.NextSibling() {
				if bl, ok := c1.(ast.Node); ok && bl.HasBlankPreviousLines() {
					list.IsTight = false
					break
				}
			}
		}
		if c != node.FirstChild() {
			if bl, ok := c.(ast.Node); ok && bl.HasBlankPreviousLines() {
				list.IsTight = false
			}
		}
	}

	if list.IsTight {
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			for gc := child.FirstChild(); gc != nil; gc = gc.NextSibling() {
				paragraph, ok := gc.(*ast.Paragraph)
				if ok {
					textBlock := ast.NewTextBlock()
					textBlock.SetLines(paragraph.Lines())
					child.ReplaceChild(child, paragraph, textBlock)
				}
			}
		}
	}
}

func (b *listParser) CanInterruptParagraph() bool {
	return true
}

func (b *listParser) CanAcceptIndentedLine() bool {
	return false
}
