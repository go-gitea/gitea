package org

import (
	"regexp"
	"strings"
)

type Paragraph struct{ Children []Node }
type HorizontalRule struct{}

var horizontalRuleRegexp = regexp.MustCompile(`^(\s*)-{5,}\s*$`)
var plainTextRegexp = regexp.MustCompile(`^(\s*)(.*)`)

func lexText(line string) (token, bool) {
	if m := plainTextRegexp.FindStringSubmatch(line); m != nil {
		return token{"text", len(m[1]), m[2], m}, true
	}
	return nilToken, false
}

func lexHorizontalRule(line string) (token, bool) {
	if m := horizontalRuleRegexp.FindStringSubmatch(line); m != nil {
		return token{"horizontalRule", len(m[1]), "", m}, true
	}
	return nilToken, false
}

func (d *Document) parseParagraph(i int, parentStop stopFn) (int, Node) {
	lines, start := []string{d.tokens[i].content}, i
	i++
	stop := func(d *Document, i int) bool {
		return parentStop(d, i) || d.tokens[i].kind != "text" || d.tokens[i].content == ""
	}
	for ; !stop(d, i); i++ {
		lines = append(lines, d.tokens[i].content)
	}
	consumed := i - start
	return consumed, Paragraph{d.parseInline(strings.Join(lines, "\n"))}
}

func (d *Document) parseHorizontalRule(i int, parentStop stopFn) (int, Node) {
	return 1, HorizontalRule{}
}

func (n Paragraph) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n HorizontalRule) String() string { return orgWriter.WriteNodesAsString(n) }
