// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package htmldiff marks the changes between two rendered HTML documents in the structure of the new one.
package htmldiff

import (
	"html/template"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gitea.dev/modules/util"

	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	classAdded   = "rich-diff-added"
	classRemoved = "rich-diff-removed"
	classChanged = "rich-diff-changed"

	documentDiffTimeout = 2 * time.Second
)

var blockClasses = map[string]string{
	classAdded:   "rich-diff-block-added",
	classRemoved: "rich-diff-block-removed",
	classChanged: "rich-diff-block-changed",
}

// Diff marks the changes from oldHTML in newHTML, the inputs must be sanitized already.
// normalizeLink rewrites links before comparing, e.g. to drop commit IDs.
func Diff(oldHTML, newHTML string, normalizeLink func(string) string) (template.HTML, error) {
	oldNodes, err := parseFragment(oldHTML)
	if err != nil {
		return "", err
	}
	newNodes, err := parseFragment(newHTML)
	if err != nil {
		return "", err
	}
	d := &differ{normalizeLink: normalizeLink, dmp: diffmatchpatch.New(), keys: map[*html.Node]string{}, deadline: time.Now().Add(documentDiffTimeout)}

	var sb strings.Builder
	for _, result := range d.diffBlocks(oldNodes, newNodes, false) {
		nodes := result.nodes
		if result.class != "" {
			nodes = []*html.Node{appendChildren(newElement(atom.Div, "rich-diff-block "+blockClasses[result.class]), nodes)}
		}
		for _, node := range nodes {
			if err := html.Render(&sb, node); err != nil {
				return "", err
			}
		}
	}
	if time.Now().After(d.deadline) {
		return "", util.ErrContentTooLarge
	}
	return template.HTML(sb.String()), nil
}

func parseFragment(fragment string) ([]*html.Node, error) {
	return html.ParseFragment(strings.NewReader(fragment), &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body})
}

type differ struct {
	normalizeLink func(string) string
	dmp           *diffmatchpatch.DiffMatchPatch
	keys          map[*html.Node]string

	deadline time.Time
}

type unit struct {
	nodes []*html.Node
	key   string
}

func (u unit) inline() bool { return !isBlock(u.nodes[0]) }

type block struct {
	nodes []*html.Node
	class string
}

func (d *differ) units(nodes []*html.Node) (ret []unit) {
	var run []*html.Node
	flush := func() {
		if !isBlank(run) {
			ret = append(ret, unit{nodes: run, key: strings.Trim(strings.Join(keysOf(run, d.nodeKey), ""), htmlSpace)})
		}
		run = nil
	}
	for _, node := range nodes {
		switch {
		case node.Type == html.CommentNode:
		case isBlock(node):
			flush()
			ret = append(ret, unit{nodes: []*html.Node{node}, key: d.nodeKey(node)})
		default:
			run = append(run, node)
		}
	}
	flush()
	return ret
}

func (d *differ) diffBlocks(oldNodes, newNodes []*html.Node, byKind bool) (ret []block) {
	oldUnits, newUnits := d.units(oldNodes), d.units(newNodes)
	var removed, added []unit
	flush := func() {
		r, a := 0, 0
		emitUntil := func(removedEnd, addedEnd int) {
			for ; r < removedEnd; r++ {
				ret = append(ret, copyBlock(removed[r], classRemoved))
			}
			for ; a < addedEnd; a++ {
				ret = append(ret, copyBlock(added[a], classAdded))
			}
		}
		for _, pair := range d.pairUnits(removed, added, byKind) {
			emitUntil(pair[0], pair[1])
			ret = append(ret, d.diffUnit(removed[r], added[a])...)
			r, a = r+1, a+1
		}
		emitUntil(len(removed), len(added))
		removed, added = nil, nil
	}
	if byKind {
		removed, added = oldUnits, newUnits
	} else {
		d.align(keysOf(oldUnits, unit.getKey), keysOf(newUnits, unit.getKey), false, func(typ diffmatchpatch.Operation, oldIdx, newIdx int) {
			switch typ {
			case diffmatchpatch.DiffEqual:
				flush()
				ret = append(ret, copyBlock(newUnits[newIdx], ""))
			case diffmatchpatch.DiffDelete:
				removed = append(removed, oldUnits[oldIdx])
			case diffmatchpatch.DiffInsert:
				added = append(added, newUnits[newIdx])
			}
		})
	}
	flush()
	return ret
}

// pairing by shared content keeps an inserted block from taking the place of an edited one
func (d *differ) pairUnits(removed, added []unit, byKind bool) (pairs [][2]int) {
	if len(removed) == 0 || len(added) == 0 {
		return nil
	}
	if len(removed) == 1 && len(added) == 1 {
		return [][2]int{{0, 0}} // diffUnit replaces them if nothing is shared
	}
	if byKind {
		next := 0
		for i, removedUnit := range removed {
			if j := slices.IndexFunc(added[next:], func(addedUnit unit) bool { return sameKind(removedUnit.nodes[0], addedUnit.nodes[0]) }); j >= 0 {
				pairs, next = append(pairs, [2]int{i, next + j}), next+j+1
			}
		}
		return pairs
	}
	type sharedContent struct {
		pair  [2]int
		count int
	}
	var shared []sharedContent // in order on both sides, like the alignment
	oldContents, oldOwners := d.contents(removed)
	newContents, newOwners := d.contents(added)
	d.align(oldContents, newContents, false, func(typ diffmatchpatch.Operation, oldIdx, newIdx int) {
		if typ != diffmatchpatch.DiffEqual {
			return
		}
		pair := [2]int{oldOwners[oldIdx], newOwners[newIdx]}
		if last := len(shared) - 1; last >= 0 && shared[last].pair == pair {
			shared[last].count++
		} else {
			shared = append(shared, sharedContent{pair: pair, count: 1})
		}
	})
	bestCount := 0
	for _, entry := range shared {
		last := len(pairs) - 1
		switch {
		case last >= 0 && entry.pair[0] == pairs[last][0]:
			if entry.count > bestCount {
				pairs[last][1], bestCount = entry.pair[1], entry.count
			}
		case last < 0 || entry.pair[1] > pairs[last][1]:
			pairs, bestCount = append(pairs, entry.pair), entry.count
		}
	}
	return pairs
}

func (d *differ) contents(units []unit) (keys []string, owners []int) {
	for owner, item := range units {
		for _, tok := range d.tokenize(item.nodes, false) {
			if isContent(&tok) {
				keys, owners = append(keys, tok.identity), append(owners, owner)
			}
		}
	}
	return keys, owners
}

func (d *differ) diffUnit(oldUnit, newUnit unit) []block {
	replace := func() []block {
		return []block{copyBlock(oldUnit, classRemoved), copyBlock(newUnit, classAdded)}
	}
	if oldUnit.key == newUnit.key {
		return []block{copyBlock(newUnit, "")}
	}
	oldNode, newNode := oldUnit.nodes[0], newUnit.nodes[0]
	if oldUnit.inline() || newUnit.inline() {
		oldContent, newContent := paragraphContent(oldUnit), paragraphContent(newUnit)
		if oldContent == nil || newContent == nil {
			return replace()
		}
		nodes, common, changed := d.diffInline(oldContent, newContent)
		if !common {
			return replace()
		}
		if !newUnit.inline() {
			nodes = []*html.Node{appendChildren(cloneNode(newNode, true), nodes)}
		}
		return []block{{nodes: nodes, class: util.Iif(changed, classChanged, "")}}
	}

	isGrid := newNode.DataAtom == atom.Table || newNode.DataAtom == atom.Tr
	if !sameKind(oldNode, newNode) || isAtomic(oldNode) || isAtomic(newNode) || isGrid && columns(oldNode) != columns(newNode) {
		return replace()
	}
	ret := cloneNode(newNode, true)
	changed := d.startTagKey(oldNode) != d.startTagKey(newNode)
	if changed {
		addClass(ret, classChanged)
	}
	oldChildren, newChildren := childNodes(oldNode), childNodes(newNode)
	switch {
	case slices.ContainsFunc(oldChildren, isBlock) || slices.ContainsFunc(newChildren, isBlock):
		for _, result := range d.diffBlocks(oldChildren, newChildren, isGrid) {
			changed = changed || result.class != ""
			appendChildren(ret, markBlock(result))
		}
	case newNode.DataAtom == atom.Pre:
		nodes, linesChanged := d.diffLines(oldChildren, newChildren)
		changed = changed || linesChanged
		appendChildren(ret, nodes)
	default:
		nodes, common, inlineChanged := d.diffInline(oldChildren, newChildren)
		if !common && !isCell(newNode) { // a replaced cell would add a column
			return replace()
		}
		changed = changed || inlineChanged
		appendChildren(ret, nodes)
	}
	return []block{{nodes: []*html.Node{ret}, class: util.Iif(changed, classChanged, "")}}
}

func paragraphContent(u unit) []*html.Node {
	if u.inline() {
		return u.nodes
	}
	if u.nodes[0].DataAtom == atom.P {
		return childNodes(u.nodes[0])
	}
	return nil
}

type token struct {
	text     string
	atomic   *html.Node
	chain    []*html.Node // inline ancestors, outermost first
	sigs     []string
	identity string // compared regardless of the formatting
	full     string // the identity including all attributes of an atomic element
}

// key is only built when comparing, a key per token would grow with the nesting depth
func (t token) key() string { return strings.Join(t.sigs, "") + t.full }

var (
	wordRegexp       = regexp.MustCompile(`(?s)[\p{L}\p{N}\p{M}_]*\p{N}(?:[.,]\p{N}+)+|[\p{Han}\p{Hiragana}\p{Katakana}]|[\p{L}\p{N}\p{M}_]+|\s+|.`)
	whitespaceRegexp = regexp.MustCompile(`\s+`)
)

// identityAttrs tell elements apart, other attribute changes like "checked" are edits in place
var identityAttrs = map[atom.Atom]string{atom.Img: "src", atom.Input: "type"}

func (d *differ) tokenize(nodes []*html.Node, lines bool) (ret []token) {
	var walk func(node *html.Node, chain []*html.Node, sigs []string)
	walk = func(node *html.Node, chain []*html.Node, sigs []string) {
		switch {
		case node.Type == html.TextNode:
			parts := strings.Lines(node.Data)
			if !lines {
				parts = slices.Values(wordRegexp.FindAllString(node.Data, -1))
			}
			for part := range parts {
				identity := "\x00" + util.Iif(!lines && isSpace(part), " ", part)
				ret = append(ret, token{text: part, chain: chain, sigs: sigs, identity: identity, full: identity})
			}
		case node.Type != html.ElementNode:
		case isAtomic(node) || node.FirstChild == nil:
			tok := token{atomic: node, chain: chain, sigs: sigs, full: "\x01" + d.nodeKey(node)}
			tok.identity = tok.full
			if key, ok := identityAttrs[node.DataAtom]; ok {
				tok.identity = "\x01" + node.Data + " " + d.attrKey(html.Attribute{Key: key, Val: attrValue(node, key)})
			}
			ret = append(ret, tok)
		default:
			chain, sigs = append(slices.Clip(chain), node), append(slices.Clip(sigs), d.startTagKey(node))
			for child := range node.ChildNodes() {
				walk(child, chain, sigs)
			}
		}
	}
	for _, node := range nodes {
		walk(node, nil, nil)
	}
	return ret
}

// diffInline returns common false if no words are shared, then a whole replacement reads better
func (d *differ) diffInline(oldNodes, newNodes []*html.Node) (nodes []*html.Node, common, changed bool) {
	oldTokens, newTokens := trimSpace(d.tokenize(oldNodes, false)), trimSpace(d.tokenize(newNodes, false))
	b := newInlineBuilder()
	var removed, added []*token
	flush := func() {
		d.align(keysOf(removed, (*token).getIdentity), keysOf(added, (*token).getIdentity), true, func(typ diffmatchpatch.Operation, oldIdx, newIdx int) {
			switch typ {
			case diffmatchpatch.DiffEqual: // only the formatting changed
				common = common || isContent(added[newIdx])
				b.add(nil, added[newIdx], classChanged)
			case diffmatchpatch.DiffDelete:
				b.add(removed[oldIdx], nil, classRemoved)
			case diffmatchpatch.DiffInsert:
				b.add(nil, added[newIdx], classAdded)
			}
		})
		changed = changed || len(removed)+len(added) > 0
		removed, added = nil, nil
	}
	d.align(keysOf(oldTokens, token.key), keysOf(newTokens, token.key), true, func(typ diffmatchpatch.Operation, oldIdx, newIdx int) {
		switch typ {
		case diffmatchpatch.DiffEqual:
			flush()
			common = common || isContent(&newTokens[newIdx])
			b.add(&oldTokens[oldIdx], &newTokens[newIdx], "")
		case diffmatchpatch.DiffDelete:
			removed = append(removed, &oldTokens[oldIdx])
		case diffmatchpatch.DiffInsert:
			added = append(added, &newTokens[newIdx])
		}
	})
	flush()
	return b.finish(), common, changed
}

// whitespace at a block's edges is not rendered
func trimSpace(tokens []token) []token {
	isSpaceToken := func(tok token) bool { return tok.atomic == nil && isSpace(tok.text) }
	for len(tokens) > 0 && isSpaceToken(tokens[0]) {
		tokens = tokens[1:]
	}
	for len(tokens) > 0 && isSpaceToken(tokens[len(tokens)-1]) {
		tokens = tokens[:len(tokens)-1]
	}
	return tokens
}

func isContent(tok *token) bool {
	return tok.atomic != nil || isWord(tok.text)
}

func (d *differ) diffLines(oldNodes, newNodes []*html.Node) (nodes []*html.Node, changed bool) {
	oldLines, oldKeys := d.lines(oldNodes)
	newLines, newKeys := d.lines(newNodes)
	b := newInlineBuilder()
	d.align(oldKeys, newKeys, false, func(typ diffmatchpatch.Operation, oldIdx, newIdx int) {
		changed = changed || typ != diffmatchpatch.DiffEqual
		if typ == diffmatchpatch.DiffDelete {
			for i := range oldLines[oldIdx] {
				b.add(&oldLines[oldIdx][i], nil, classRemoved)
			}
			return
		}
		class := util.Iif(typ == diffmatchpatch.DiffInsert, classAdded, "")
		for i := range newLines[newIdx] {
			b.add(nil, &newLines[newIdx][i], class)
		}
	})
	return b.finish(), changed
}

// lines groups preformatted tokens by line, keyed by their text as highlighting may split a line differently
func (d *differ) lines(nodes []*html.Node) (lines [][]token, keys []string) {
	tokens := d.tokenize(nodes, true)
	var key strings.Builder
	start := 0
	for i, tok := range tokens {
		key.WriteString(util.Iif(tok.atomic == nil, tok.text, tok.identity))
		if strings.HasSuffix(tok.text, "\n") || i == len(tokens)-1 {
			lines, keys = append(lines, tokens[start:i+1]), append(keys, key.String())
			start = i + 1
			key.Reset()
		}
	}
	return lines, keys
}

type inlineBuilder struct {
	root    *html.Node
	stack   []openElement
	mark    *html.Node
	text    *html.Node
	textBuf strings.Builder // appending to the text node for each token would be quadratic
}

type openElement struct {
	node    *html.Node
	sig     string
	sources [2]*html.Node // the old and new element, equal siblings must not be merged
}

func newInlineBuilder() *inlineBuilder {
	return &inlineBuilder{root: &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}}
}

func (b *inlineBuilder) add(oldToken, newToken *token, class string) {
	sides := [2]*token{oldToken, newToken}
	tok := util.IfZero(newToken, oldToken)
	isOpen := func(i int) bool {
		el := b.stack[i]
		for side, sideToken := range sides {
			if sideToken != nil && el.sources[side] != nil && el.sources[side] != sideToken.chain[i] {
				return false
			}
		}
		return el.sig == tok.sigs[i]
	}
	common := 0
	for common < len(b.stack) && common < len(tok.sigs) && isOpen(common) {
		common++
	}
	b.stack = b.stack[:common]
	for i := common; i < len(tok.chain); i++ {
		el := cloneNode(tok.chain[i], newToken != nil)
		b.parent().AppendChild(el)
		b.stack = append(b.stack, openElement{node: el, sig: tok.sigs[i]})
	}
	for i := range b.stack {
		if i < common && newToken != nil && b.stack[i].sources[1] == nil {
			b.stack[i].node.Attr = slices.Clone(newToken.chain[i].Attr) // opened by a removed token without its id
		}
		for side, sideToken := range sides {
			if sideToken != nil {
				b.stack[i].sources[side] = sideToken.chain[i]
			}
		}
	}

	target := b.parent()
	if class != "" {
		if b.mark == nil || target.LastChild != b.mark || attrValue(b.mark, "class") != class {
			b.mark = newMark(class)
			target.AppendChild(b.mark)
		}
		target = b.mark
	}
	if tok.atomic != nil {
		target.AppendChild(cloneTree(tok.atomic, newToken != nil))
		return
	}
	if b.text == nil || target.LastChild != b.text {
		b.flushText()
		b.text = &html.Node{Type: html.TextNode}
		target.AppendChild(b.text)
	}
	b.textBuf.WriteString(tok.text)
}

func (b *inlineBuilder) flushText() {
	if b.text != nil {
		b.text.Data = b.textBuf.String()
		b.textBuf.Reset()
		b.text = nil
	}
}

func (b *inlineBuilder) parent() *html.Node {
	if len(b.stack) == 0 {
		return b.root
	}
	return b.stack[len(b.stack)-1].node
}

func (b *inlineBuilder) finish() []*html.Node {
	b.flushText()
	ret := childNodes(b.root)
	for _, child := range ret {
		b.root.RemoveChild(child)
	}
	return ret
}

func (d *differ) align(oldKeys, newKeys []string, cleanup bool, fn func(typ diffmatchpatch.Operation, oldIdx, newIdx int)) {
	oldIdx, newIdx := 0, 0
	emit := func(typ diffmatchpatch.Operation, count int) {
		for range count {
			fn(typ, oldIdx, newIdx)
			if typ != diffmatchpatch.DiffInsert {
				oldIdx++
			}
			if typ != diffmatchpatch.DiffDelete {
				newIdx++
			}
		}
	}
	d.dmp.DiffTimeout = time.Until(d.deadline)
	if d.dmp.DiffTimeout <= 0 || len(oldKeys) == 0 || len(newKeys) == 0 {
		emit(diffmatchpatch.DiffDelete, len(oldKeys))
		emit(diffmatchpatch.DiffInsert, len(newKeys))
		return
	}
	runeOf := map[string]rune{}
	toRunes := func(keys []string) []rune {
		ret := make([]rune, len(keys))
		for i, key := range keys {
			char, ok := runeOf[key]
			if !ok {
				char = rune(len(runeOf) + utf8.RuneSelf) // ASCII would make the semantic cleanup see word boundaries
				if char >= 0xD800 {
					char += 0x800 // skip the surrogates, they can't be encoded in the diff texts
				}
				runeOf[key] = char
			}
			ret[i] = char
		}
		return ret
	}
	diffs := d.dmp.DiffMainRunes(toRunes(oldKeys), toRunes(newKeys), false)
	if cleanup {
		diffs = d.dmp.DiffCleanupSemantic(diffs)
	}
	for _, diff := range diffs {
		emit(diff.Type, utf8.RuneCountInString(diff.Text))
	}
}

func keysOf[T any](items []T, key func(T) string) []string {
	ret := make([]string, len(items))
	for i, item := range items {
		ret[i] = key(item)
	}
	return ret
}

func (u unit) getKey() string { return u.key }

func (t token) getIdentity() string { return t.identity }

// copyBlock drops the ids of a removed copy, anchors must not jump to it
func copyBlock(u unit, class string) block {
	nodes := make([]*html.Node, len(u.nodes))
	for i, node := range u.nodes {
		nodes[i] = cloneTree(node, class != classRemoved)
	}
	return block{nodes: nodes, class: class}
}

func markBlock(result block) []*html.Node {
	if result.class == "" || result.class == classChanged {
		return result.nodes
	}
	if !isBlock(result.nodes[0]) {
		return []*html.Node{appendChildren(newMark(result.class), result.nodes)}
	}
	if isAtomic(result.nodes[0]) {
		// client-side renderers (math, mermaid) replace the element, so the class must be on a wrapper
		return []*html.Node{appendChildren(newElement(atom.Div, result.class), result.nodes)}
	}
	addClass(result.nodes[0], result.class)
	return result.nodes
}

// nodeKey serializes a node once, the keys of its descendants are substrings of it
func (d *differ) nodeKey(node *html.Node) string {
	if key, ok := d.keys[node]; ok {
		return key
	}
	type nodeSpan struct {
		node       *html.Node
		start, end int
	}
	var sb strings.Builder
	var spans []nodeSpan
	var write func(current *html.Node)
	write = func(current *html.Node) {
		start := sb.Len()
		switch current.Type {
		case html.TextNode:
			sb.WriteString(html.EscapeString(normalizeSpace(current)))
		case html.ElementNode:
			sb.WriteString(d.startTagKey(current))
			for child := range current.ChildNodes() {
				write(child)
			}
			sb.WriteString("</" + current.Data + ">")
		}
		spans = append(spans, nodeSpan{current, start, sb.Len()})
	}
	write(node)
	key := sb.String()
	for _, span := range spans {
		d.keys[span.node] = key[span.start:span.end]
	}
	return key
}

func (d *differ) startTagKey(node *html.Node) string {
	var sb strings.Builder
	sb.WriteString("<" + node.Data)
	for _, attr := range node.Attr {
		if attr.Key != "id" && attr.Key != "data-source-position" { // ids follow the content
			sb.WriteString(" " + attr.Key + `="` + html.EscapeString(d.attrKey(attr)) + `"`)
		}
	}
	sb.WriteString(">")
	return sb.String()
}

func (d *differ) attrKey(attr html.Attribute) string {
	if (attr.Key == "href" || attr.Key == "src" || attr.Key == "srcset") && d.normalizeLink != nil {
		return d.normalizeLink(attr.Val)
	}
	return attr.Val
}

// whitespace runs render alike outside preformatted text
func normalizeSpace(textNode *html.Node) string {
	for node := textNode.Parent; node != nil; node = node.Parent {
		if node.DataAtom == atom.Pre {
			return textNode.Data
		}
	}
	return whitespaceRegexp.ReplaceAllString(textNode.Data, " ")
}

var blockAtoms = map[atom.Atom]bool{
	atom.Address: true, atom.Article: true, atom.Aside: true, atom.Blockquote: true, atom.Caption: true, atom.Colgroup: true,
	atom.Dd: true, atom.Details: true, atom.Div: true, atom.Dl: true, atom.Dt: true, atom.Figcaption: true,
	atom.Figure: true, atom.Footer: true, atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true,
	atom.H5: true, atom.H6: true, atom.Header: true, atom.Hr: true, atom.Li: true, atom.Ol: true,
	atom.P: true, atom.Pre: true, atom.Section: true, atom.Table: true, atom.Tbody: true, atom.Td: true,
	atom.Tfoot: true, atom.Th: true, atom.Thead: true, atom.Tr: true, atom.Ul: true,
}

var atomicAtoms = map[atom.Atom]bool{
	atom.Audio: true, atom.Br: true, atom.Canvas: true, atom.Details: true, atom.Embed: true, atom.Hr: true,
	atom.Iframe: true, atom.Img: true, atom.Input: true, atom.Kbd: true, atom.Math: true, atom.Object: true,
	atom.Picture: true, atom.Svg: true, atom.Video: true,
}

func isBlock(node *html.Node) bool {
	return node.Type == html.ElementNode && blockAtoms[node.DataAtom]
}

func sameKind(oldNode, newNode *html.Node) bool {
	return oldNode.Data == newNode.Data || isHeading(oldNode) && isHeading(newNode) || isCell(oldNode) && isCell(newNode)
}

func isHeading(node *html.Node) bool {
	switch node.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		return true
	}
	return false
}

// columns returns the number of cells in a row, or in the first row of a table
func columns(node *html.Node) int {
	if node.DataAtom == atom.Table {
		for row := range node.Descendants() {
			if row.DataAtom == atom.Tr {
				return columns(row)
			}
		}
	}
	return len(slices.DeleteFunc(childNodes(node), func(child *html.Node) bool { return !isCell(child) }))
}

func isCell(node *html.Node) bool {
	return node.DataAtom == atom.Td || node.DataAtom == atom.Th
}

func isAtomic(node *html.Node) bool {
	if node.Type != html.ElementNode {
		return false
	}
	isInlineCode := node.DataAtom == atom.Code && (node.Parent == nil || node.Parent.DataAtom != atom.Pre)
	isEmoji := node.DataAtom == atom.Span && attrValue(node, "class") == "emoji" // a sequence like a flag must not be split
	return atomicAtoms[node.DataAtom] || isInlineCode || isEmoji || isClientRenderedCode(node)
}

// isClientRenderedCode reports math and mermaid code, mixed old and new source would render wrong
func isClientRenderedCode(node *html.Node) bool {
	switch node.DataAtom {
	case atom.Code:
		classes := strings.Fields(attrValue(node, "class"))
		return slices.Contains(classes, "language-math") || slices.Contains(classes, "language-mermaid")
	case atom.Pre, atom.Div:
		for child := range node.ChildNodes() {
			if isClientRenderedCode(child) {
				return true
			}
		}
	}
	return false
}

const htmlSpace = " \t\n\f\r"

func isSpace(text string) bool {
	return strings.Trim(text, htmlSpace) == ""
}

func isWord(text string) bool {
	first, _ := utf8.DecodeRuneInString(text)
	return unicode.IsLetter(first) || unicode.IsNumber(first)
}

func isBlank(nodes []*html.Node) bool {
	for _, node := range nodes {
		if node.Type != html.TextNode || !isSpace(node.Data) {
			return false
		}
	}
	return true
}

func childNodes(node *html.Node) []*html.Node {
	return slices.Collect(node.ChildNodes())
}

func appendChildren(node *html.Node, children []*html.Node) *html.Node {
	for _, child := range children {
		node.AppendChild(child)
	}
	return node
}

func cloneNode(node *html.Node, keepIDs bool) *html.Node {
	attrs := slices.Clone(node.Attr)
	if !keepIDs {
		attrs = slices.DeleteFunc(attrs, func(attr html.Attribute) bool { return attr.Key == "id" })
	}
	return &html.Node{Type: node.Type, DataAtom: node.DataAtom, Data: node.Data, Namespace: node.Namespace, Attr: attrs}
}

func cloneTree(node *html.Node, keepIDs bool) *html.Node {
	ret := cloneNode(node, keepIDs)
	for child := range node.ChildNodes() {
		ret.AppendChild(cloneTree(child, keepIDs))
	}
	return ret
}

func newMark(class string) *html.Node {
	switch class {
	case classAdded:
		return newElement(atom.Ins, class)
	case classRemoved:
		return newElement(atom.Del, class)
	}
	return newElement(atom.Span, class)
}

func newElement(tag atom.Atom, class string) *html.Node {
	return &html.Node{Type: html.ElementNode, DataAtom: tag, Data: tag.String(), Attr: []html.Attribute{{Key: "class", Val: class}}}
}

func addClass(node *html.Node, class string) {
	for i, attr := range node.Attr {
		if attr.Key == "class" {
			node.Attr[i].Val = strings.TrimSpace(attr.Val + " " + class)
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: "class", Val: class})
}

func attrValue(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
