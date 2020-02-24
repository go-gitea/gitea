package org

import (
	"regexp"
)

type FootnoteDefinition struct {
	Name     string
	Children []Node
	Inline   bool
}

var footnoteDefinitionRegexp = regexp.MustCompile(`^\[fn:([\w-]+)\](\s+(.+)|\s*$)`)

func lexFootnoteDefinition(line string) (token, bool) {
	if m := footnoteDefinitionRegexp.FindStringSubmatch(line); m != nil {
		return token{"footnoteDefinition", 0, m[1], m}, true
	}
	return nilToken, false
}

func (d *Document) parseFootnoteDefinition(i int, parentStop stopFn) (int, Node) {
	start, name := i, d.tokens[i].content
	d.tokens[i] = tokenize(d.tokens[i].matches[2])
	stop := func(d *Document, i int) bool {
		return parentStop(d, i) ||
			(isSecondBlankLine(d, i) && i > start+1) ||
			d.tokens[i].kind == "headline" || d.tokens[i].kind == "footnoteDefinition"
	}
	consumed, nodes := d.parseMany(i, stop)
	definition := FootnoteDefinition{name, nodes, false}
	return consumed, definition
}

func (n FootnoteDefinition) String() string { return orgWriter.WriteNodesAsString(n) }
