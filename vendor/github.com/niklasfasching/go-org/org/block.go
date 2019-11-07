package org

import (
	"regexp"
	"strings"
	"unicode"
)

type Block struct {
	Name       string
	Parameters []string
	Children   []Node
}

type Example struct {
	Children []Node
}

var exampleLineRegexp = regexp.MustCompile(`^(\s*):(\s(.*)|\s*$)`)
var beginBlockRegexp = regexp.MustCompile(`(?i)^(\s*)#\+BEGIN_(\w+)(.*)`)
var endBlockRegexp = regexp.MustCompile(`(?i)^(\s*)#\+END_(\w+)`)

func lexBlock(line string) (token, bool) {
	if m := beginBlockRegexp.FindStringSubmatch(line); m != nil {
		return token{"beginBlock", len(m[1]), strings.ToUpper(m[2]), m}, true
	} else if m := endBlockRegexp.FindStringSubmatch(line); m != nil {
		return token{"endBlock", len(m[1]), strings.ToUpper(m[2]), m}, true
	}
	return nilToken, false
}

func lexExample(line string) (token, bool) {
	if m := exampleLineRegexp.FindStringSubmatch(line); m != nil {
		return token{"example", len(m[1]), m[3], m}, true
	}
	return nilToken, false
}

func isRawTextBlock(name string) bool { return name == "SRC" || name == "EXAMPLE" || name == "EXPORT" }

func (d *Document) parseBlock(i int, parentStop stopFn) (int, Node) {
	t, start := d.tokens[i], i
	name, parameters := t.content, strings.Fields(t.matches[3])
	trim := trimIndentUpTo(d.tokens[i].lvl)
	stop := func(d *Document, i int) bool {
		return i >= len(d.tokens) || (d.tokens[i].kind == "endBlock" && d.tokens[i].content == name)
	}
	block, i := Block{name, parameters, nil}, i+1
	if isRawTextBlock(name) {
		rawText := ""
		for ; !stop(d, i); i++ {
			rawText += trim(d.tokens[i].matches[0]) + "\n"
		}
		block.Children = d.parseRawInline(rawText)
	} else {
		consumed, nodes := d.parseMany(i, stop)
		block.Children = nodes
		i += consumed
	}
	if i < len(d.tokens) && d.tokens[i].kind == "endBlock" && d.tokens[i].content == name {
		return i + 1 - start, block
	}
	return 0, nil
}

func (d *Document) parseExample(i int, parentStop stopFn) (int, Node) {
	example, start := Example{}, i
	for ; !parentStop(d, i) && d.tokens[i].kind == "example"; i++ {
		example.Children = append(example.Children, Text{d.tokens[i].content, true})
	}
	return i - start, example
}

func trimIndentUpTo(max int) func(string) string {
	return func(line string) string {
		i := 0
		for ; i < len(line) && i < max && unicode.IsSpace(rune(line[i])); i++ {
		}
		return line[i:]
	}
}

func (n Example) String() string { return orgWriter.WriteNodesAsString(n) }
func (n Block) String() string   { return orgWriter.WriteNodesAsString(n) }
