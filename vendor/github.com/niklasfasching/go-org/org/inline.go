package org

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type Text struct {
	Content string
	IsRaw   bool
}

type LineBreak struct{ Count int }
type ExplicitLineBreak struct{}

type StatisticToken struct{ Content string }

type Timestamp struct {
	Time     time.Time
	IsDate   bool
	Interval string
}

type Emphasis struct {
	Kind    string
	Content []Node
}

type InlineBlock struct {
	Name       string
	Parameters []string
	Children   []Node
}

type LatexFragment struct {
	OpeningPair string
	ClosingPair string
	Content     []Node
}

type FootnoteLink struct {
	Name       string
	Definition *FootnoteDefinition
}

type RegularLink struct {
	Protocol    string
	Description []Node
	URL         string
	AutoLink    bool
}

type Macro struct {
	Name       string
	Parameters []string
}

var validURLCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~:/?#[]@!$&'()*+,;="
var autolinkProtocols = regexp.MustCompile(`^(https?|ftp|file)$`)
var imageExtensionRegexp = regexp.MustCompile(`^[.](png|gif|jpe?g|svg|tiff?)$`)
var videoExtensionRegexp = regexp.MustCompile(`^[.](webm|mp4)$`)

var subScriptSuperScriptRegexp = regexp.MustCompile(`^([_^]){([^{}]+?)}`)
var timestampRegexp = regexp.MustCompile(`^<(\d{4}-\d{2}-\d{2})( [A-Za-z]+)?( \d{2}:\d{2})?( \+\d+[dwmy])?>`)
var footnoteRegexp = regexp.MustCompile(`^\[fn:([\w-]*?)(:(.*?))?\]`)
var statisticsTokenRegexp = regexp.MustCompile(`^\[(\d+/\d+|\d+%)\]`)
var latexFragmentRegexp = regexp.MustCompile(`(?s)^\\begin{(\w+)}(.*)\\end{(\w+)}`)
var inlineBlockRegexp = regexp.MustCompile(`src_(\w+)(\[(.*)\])?{(.*)}`)
var inlineExportBlockRegexp = regexp.MustCompile(`@@(\w+):(.*?)@@`)
var macroRegexp = regexp.MustCompile(`{{{(.*)\((.*)\)}}}`)

var timestampFormat = "2006-01-02 Mon 15:04"
var datestampFormat = "2006-01-02 Mon"

var latexFragmentPairs = map[string]string{
	`\(`: `\)`,
	`\[`: `\]`,
	`$$`: `$$`,
	`$`:  `$`,
}

func (d *Document) parseInline(input string) (nodes []Node) {
	previous, current := 0, 0
	for current < len(input) {
		rewind, consumed, node := 0, 0, (Node)(nil)
		switch input[current] {
		case '^':
			consumed, node = d.parseSubOrSuperScript(input, current)
		case '_':
			rewind, consumed, node = d.parseSubScriptOrEmphasisOrInlineBlock(input, current)
		case '@':
			consumed, node = d.parseInlineExportBlock(input, current)
		case '*', '/', '+':
			consumed, node = d.parseEmphasis(input, current, false)
		case '=', '~':
			consumed, node = d.parseEmphasis(input, current, true)
		case '[':
			consumed, node = d.parseOpeningBracket(input, current)
		case '{':
			consumed, node = d.parseMacro(input, current)
		case '<':
			consumed, node = d.parseTimestamp(input, current)
		case '\\':
			consumed, node = d.parseExplicitLineBreakOrLatexFragment(input, current)
		case '$':
			consumed, node = d.parseLatexFragment(input, current, 1)
		case '\n':
			consumed, node = d.parseLineBreak(input, current)
		case ':':
			rewind, consumed, node = d.parseAutoLink(input, current)
		}
		current -= rewind
		if consumed != 0 {
			if current > previous {
				nodes = append(nodes, Text{input[previous:current], false})
			}
			if node != nil {
				nodes = append(nodes, node)
			}
			current += consumed
			previous = current
		} else {
			current++
		}
	}

	if previous < len(input) {
		nodes = append(nodes, Text{input[previous:], false})
	}
	return nodes
}

func (d *Document) parseRawInline(input string) (nodes []Node) {
	previous, current := 0, 0
	for current < len(input) {
		if input[current] == '\n' {
			consumed, node := d.parseLineBreak(input, current)
			if current > previous {
				nodes = append(nodes, Text{input[previous:current], true})
			}
			nodes = append(nodes, node)
			current += consumed
			previous = current
		} else {
			current++
		}
	}
	if previous < len(input) {
		nodes = append(nodes, Text{input[previous:], true})
	}
	return nodes
}

func (d *Document) parseLineBreak(input string, start int) (int, Node) {
	i := start
	for ; i < len(input) && input[i] == '\n'; i++ {
	}
	return i - start, LineBreak{i - start}
}

func (d *Document) parseInlineBlock(input string, start int) (int, int, Node) {
	if !(strings.HasSuffix(input[:start], "src") && (start-4 < 0 || unicode.IsSpace(rune(input[start-4])))) {
		return 0, 0, nil
	}
	if m := inlineBlockRegexp.FindStringSubmatch(input[start-3:]); m != nil {
		return 3, len(m[0]), InlineBlock{"src", strings.Fields(m[1] + " " + m[3]), d.parseRawInline(m[4])}
	}
	return 0, 0, nil
}

func (d *Document) parseInlineExportBlock(input string, start int) (int, Node) {
	if m := inlineExportBlockRegexp.FindStringSubmatch(input[start:]); m != nil {
		return len(m[0]), InlineBlock{"export", m[1:2], d.parseRawInline(m[2])}
	}
	return 0, nil
}

func (d *Document) parseExplicitLineBreakOrLatexFragment(input string, start int) (int, Node) {
	switch {
	case start+2 >= len(input):
	case input[start+1] == '\\' && start != 0 && input[start-1] != '\n':
		for i := start + 2; i <= len(input)-1 && unicode.IsSpace(rune(input[i])); i++ {
			if input[i] == '\n' {
				return i + 1 - start, ExplicitLineBreak{}
			}
		}
	case input[start+1] == '(' || input[start+1] == '[':
		return d.parseLatexFragment(input, start, 2)
	case strings.Index(input[start:], `\begin{`) == 0:
		if m := latexFragmentRegexp.FindStringSubmatch(input[start:]); m != nil {
			if open, content, close := m[1], m[2], m[3]; open == close {
				openingPair, closingPair := `\begin{`+open+`}`, `\end{`+close+`}`
				i := strings.Index(input[start:], closingPair)
				return i + len(closingPair), LatexFragment{openingPair, closingPair, d.parseRawInline(content)}
			}
		}
	}
	return 0, nil
}

func (d *Document) parseLatexFragment(input string, start int, pairLength int) (int, Node) {
	if start+2 >= len(input) {
		return 0, nil
	}
	if pairLength == 1 && input[start:start+2] == "$$" {
		pairLength = 2
	}
	openingPair := input[start : start+pairLength]
	closingPair := latexFragmentPairs[openingPair]
	if i := strings.Index(input[start+pairLength:], closingPair); i != -1 {
		content := d.parseRawInline(input[start+pairLength : start+pairLength+i])
		return i + pairLength + pairLength, LatexFragment{openingPair, closingPair, content}
	}
	return 0, nil
}

func (d *Document) parseSubOrSuperScript(input string, start int) (int, Node) {
	if m := subScriptSuperScriptRegexp.FindStringSubmatch(input[start:]); m != nil {
		return len(m[2]) + 3, Emphasis{m[1] + "{}", []Node{Text{m[2], false}}}
	}
	return 0, nil
}

func (d *Document) parseSubScriptOrEmphasisOrInlineBlock(input string, start int) (int, int, Node) {
	if rewind, consumed, node := d.parseInlineBlock(input, start); consumed != 0 {
		return rewind, consumed, node
	} else if consumed, node := d.parseSubOrSuperScript(input, start); consumed != 0 {
		return 0, consumed, node
	}
	consumed, node := d.parseEmphasis(input, start, false)
	return 0, consumed, node
}

func (d *Document) parseOpeningBracket(input string, start int) (int, Node) {
	if len(input[start:]) >= 2 && input[start] == '[' && input[start+1] == '[' {
		return d.parseRegularLink(input, start)
	} else if footnoteRegexp.MatchString(input[start:]) {
		return d.parseFootnoteReference(input, start)
	} else if statisticsTokenRegexp.MatchString(input[start:]) {
		return d.parseStatisticToken(input, start)
	}
	return 0, nil
}

func (d *Document) parseMacro(input string, start int) (int, Node) {
	if m := macroRegexp.FindStringSubmatch(input[start:]); m != nil {
		return len(m[0]), Macro{m[1], strings.Split(m[2], ",")}
	}
	return 0, nil
}

func (d *Document) parseFootnoteReference(input string, start int) (int, Node) {
	if m := footnoteRegexp.FindStringSubmatch(input[start:]); m != nil {
		name, definition := m[1], m[3]
		if name == "" && definition == "" {
			return 0, nil
		}
		link := FootnoteLink{name, nil}
		if definition != "" {
			link.Definition = &FootnoteDefinition{name, []Node{Paragraph{d.parseInline(definition)}}, true}
		}
		return len(m[0]), link
	}
	return 0, nil
}

func (d *Document) parseStatisticToken(input string, start int) (int, Node) {
	if m := statisticsTokenRegexp.FindStringSubmatch(input[start:]); m != nil {
		return len(m[1]) + 2, StatisticToken{m[1]}
	}
	return 0, nil
}

func (d *Document) parseAutoLink(input string, start int) (int, int, Node) {
	if !d.AutoLink || start == 0 || len(input[start:]) < 3 || input[start:start+3] != "://" {
		return 0, 0, nil
	}
	protocolStart, protocol := start-1, ""
	for ; protocolStart > 0; protocolStart-- {
		if !unicode.IsLetter(rune(input[protocolStart])) {
			protocolStart++
			break
		}
	}
	if m := autolinkProtocols.FindStringSubmatch(input[protocolStart:start]); m != nil {
		protocol = m[1]
	} else {
		return 0, 0, nil
	}
	end := start
	for ; end < len(input) && strings.ContainsRune(validURLCharacters, rune(input[end])); end++ {
	}
	path := input[start:end]
	if path == "://" {
		return 0, 0, nil
	}
	return len(protocol), len(path + protocol), RegularLink{protocol, nil, protocol + path, true}
}

func (d *Document) parseRegularLink(input string, start int) (int, Node) {
	input = input[start:]
	if len(input) < 3 || input[:2] != "[[" || input[2] == '[' {
		return 0, nil
	}
	end := strings.Index(input, "]]")
	if end == -1 {
		return 0, nil
	}
	rawLinkParts := strings.Split(input[2:end], "][")
	description, link := ([]Node)(nil), rawLinkParts[0]
	if len(rawLinkParts) == 2 {
		link, description = rawLinkParts[0], d.parseInline(rawLinkParts[1])
	}
	if strings.ContainsRune(link, '\n') {
		return 0, nil
	}
	consumed := end + 2
	protocol, linkParts := "", strings.SplitN(link, ":", 2)
	if len(linkParts) == 2 {
		protocol = linkParts[0]
	}
	return consumed, RegularLink{protocol, description, link, false}
}

func (d *Document) parseTimestamp(input string, start int) (int, Node) {
	if m := timestampRegexp.FindStringSubmatch(input[start:]); m != nil {
		ddmmyy, hhmm, interval, isDate := m[1], m[3], strings.TrimSpace(m[4]), false
		if hhmm == "" {
			hhmm, isDate = "00:00", true
		}
		t, err := time.Parse(timestampFormat, fmt.Sprintf("%s Mon %s", ddmmyy, hhmm))
		if err != nil {
			return 0, nil
		}
		timestamp := Timestamp{t, isDate, interval}
		return len(m[0]), timestamp
	}
	return 0, nil
}

func (d *Document) parseEmphasis(input string, start int, isRaw bool) (int, Node) {
	marker, i := input[start], start
	if !hasValidPreAndBorderChars(input, i) {
		return 0, nil
	}
	for i, consumedNewLines := i+1, 0; i < len(input) && consumedNewLines <= d.MaxEmphasisNewLines; i++ {
		if input[i] == '\n' {
			consumedNewLines++
		}

		if input[i] == marker && i != start+1 && hasValidPostAndBorderChars(input, i) {
			if isRaw {
				return i + 1 - start, Emphasis{input[start : start+1], d.parseRawInline(input[start+1 : i])}
			}
			return i + 1 - start, Emphasis{input[start : start+1], d.parseInline(input[start+1 : i])}
		}
	}
	return 0, nil
}

// see org-emphasis-regexp-components (emacs elisp variable)

func hasValidPreAndBorderChars(input string, i int) bool {
	return (i+1 >= len(input) || isValidBorderChar(rune(input[i+1]))) && (i == 0 || isValidPreChar(rune(input[i-1])))
}

func hasValidPostAndBorderChars(input string, i int) bool {
	return (i == 0 || isValidBorderChar(rune(input[i-1]))) && (i+1 >= len(input) || isValidPostChar(rune(input[i+1])))
}

func isValidPreChar(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune(`-({'"`, r)
}

func isValidPostChar(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune(`-.,:!?;'")}[`, r)
}

func isValidBorderChar(r rune) bool { return !unicode.IsSpace(r) }

func (l RegularLink) Kind() string {
	description := String(l.Description)
	descProtocol, descExt := strings.SplitN(description, ":", 2)[0], path.Ext(description)
	if ok := descProtocol == "file" || descProtocol == "http" || descProtocol == "https"; ok && imageExtensionRegexp.MatchString(descExt) {
		return "image"
	} else if ok && videoExtensionRegexp.MatchString(descExt) {
		return "video"
	}

	if p := l.Protocol; l.Description != nil || (p != "" && p != "file" && p != "http" && p != "https") {
		return "regular"
	}
	if imageExtensionRegexp.MatchString(path.Ext(l.URL)) {
		return "image"
	}
	if videoExtensionRegexp.MatchString(path.Ext(l.URL)) {
		return "video"
	}
	return "regular"
}

func (n Text) String() string              { return orgWriter.WriteNodesAsString(n) }
func (n LineBreak) String() string         { return orgWriter.WriteNodesAsString(n) }
func (n ExplicitLineBreak) String() string { return orgWriter.WriteNodesAsString(n) }
func (n StatisticToken) String() string    { return orgWriter.WriteNodesAsString(n) }
func (n Emphasis) String() string          { return orgWriter.WriteNodesAsString(n) }
func (n InlineBlock) String() string       { return orgWriter.WriteNodesAsString(n) }
func (n LatexFragment) String() string     { return orgWriter.WriteNodesAsString(n) }
func (n FootnoteLink) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n RegularLink) String() string       { return orgWriter.WriteNodesAsString(n) }
func (n Macro) String() string             { return orgWriter.WriteNodesAsString(n) }
func (n Timestamp) String() string         { return orgWriter.WriteNodesAsString(n) }
