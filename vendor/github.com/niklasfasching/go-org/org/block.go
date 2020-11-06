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
	Result     Node
}

type Result struct {
	Node Node
}

type Example struct {
	Children []Node
}

var exampleLineRegexp = regexp.MustCompile(`^(\s*):(\s(.*)|\s*$)`)
var beginBlockRegexp = regexp.MustCompile(`(?i)^(\s*)#\+BEGIN_(\w+)(.*)`)
var endBlockRegexp = regexp.MustCompile(`(?i)^(\s*)#\+END_(\w+)`)
var resultRegexp = regexp.MustCompile(`(?i)^(\s*)#\+RESULTS:`)
var exampleBlockEscapeRegexp = regexp.MustCompile(`(^|\n)([ \t]*),([ \t]*)(\*|,\*|#\+|,#\+)`)

func lexBlock(line string) (token, bool) {
	if m := beginBlockRegexp.FindStringSubmatch(line); m != nil {
		return token{"beginBlock", len(m[1]), strings.ToUpper(m[2]), m}, true
	} else if m := endBlockRegexp.FindStringSubmatch(line); m != nil {
		return token{"endBlock", len(m[1]), strings.ToUpper(m[2]), m}, true
	}
	return nilToken, false
}

func lexResult(line string) (token, bool) {
	if m := resultRegexp.FindStringSubmatch(line); m != nil {
		return token{"result", len(m[1]), "", m}, true
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
	block, i := Block{name, parameters, nil, nil}, i+1
	if isRawTextBlock(name) {
		rawText := ""
		for ; !stop(d, i); i++ {
			rawText += trim(d.tokens[i].matches[0]) + "\n"
		}
		if name == "EXAMPLE" || (name == "SRC" && len(parameters) >= 1 && parameters[0] == "org") {
			rawText = exampleBlockEscapeRegexp.ReplaceAllString(rawText, "$1$2$3$4")
		}
		block.Children = d.parseRawInline(rawText)
	} else {
		consumed, nodes := d.parseMany(i, stop)
		block.Children = nodes
		i += consumed
	}
	if i >= len(d.tokens) || d.tokens[i].kind != "endBlock" || d.tokens[i].content != name {
		return 0, nil
	}
	if name == "SRC" {
		consumed, result := d.parseSrcBlockResult(i+1, parentStop)
		block.Result = result
		i += consumed
	}
	return i + 1 - start, block
}

func (d *Document) parseSrcBlockResult(i int, parentStop stopFn) (int, Node) {
	start := i
	for ; !parentStop(d, i) && d.tokens[i].kind == "text" && d.tokens[i].content == ""; i++ {
	}
	if parentStop(d, i) || d.tokens[i].kind != "result" {
		return 0, nil
	}
	consumed, result := d.parseResult(i, parentStop)
	return (i - start) + consumed, result
}

func (d *Document) parseExample(i int, parentStop stopFn) (int, Node) {
	example, start := Example{}, i
	for ; !parentStop(d, i) && d.tokens[i].kind == "example"; i++ {
		example.Children = append(example.Children, Text{d.tokens[i].content, true})
	}
	return i - start, example
}

func (d *Document) parseResult(i int, parentStop stopFn) (int, Node) {
	if i+1 >= len(d.tokens) {
		return 0, nil
	}
	consumed, node := d.parseOne(i+1, parentStop)
	return consumed + 1, Result{node}
}

func trimIndentUpTo(max int) func(string) string {
	return func(line string) string {
		i := 0
		for ; i < len(line) && i < max && unicode.IsSpace(rune(line[i])); i++ {
		}
		return line[i:]
	}
}

func (b Block) ParameterMap() map[string]string {
	if len(b.Parameters) == 0 {
		return nil
	}
	m := map[string]string{":lang": b.Parameters[0]}
	for i := 1; i+1 < len(b.Parameters); i += 2 {
		m[b.Parameters[i]] = b.Parameters[i+1]
	}
	return m
}

func (n Example) String() string { return orgWriter.WriteNodesAsString(n) }
func (n Block) String() string   { return orgWriter.WriteNodesAsString(n) }
func (n Result) String() string  { return orgWriter.WriteNodesAsString(n) }
