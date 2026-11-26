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
	ClassAdded   = "rich-diff-added"
	ClassRemoved = "rich-diff-removed"
	ClassChanged = "rich-diff-changed"

	documentDiffTimeout = 2 * time.Second
)

type Options struct {
	NormalizeLink func(link string) string // rewrites links before comparing, e.g. to drop commit IDs
}

var blockClasses = map[string]string{
	ClassAdded:   "rich-diff-block-added",
	ClassRemoved: "rich-diff-block-removed",
	ClassChanged: "rich-diff-block-changed",
}

// Diff marks the changes from oldHTML in newHTML, the inputs must be sanitized already
func Diff(oldHTML, newHTML string, opts Options) (template.HTML, error) {
	oldNodes, err := parseFragment(oldHTML)
	if err != nil {
		return "", err
	}
	newNodes, err := parseFragment(newHTML)
	if err != nil {
		return "", err
	}
	d := &differ{opts: opts, dmp: diffmatchpatch.New(), keys: map[*html.Node]string{}, deadline: time.Now().Add(documentDiffTimeout)}

	var sb strings.Builder
	for _, b := range d.diffBlocks(oldNodes, newNodes, false) {
		nodes := b.nodes
		if b.class != "" {
			nodes = []*html.Node{appendChildren(newElement(atom.Div, "rich-diff-block "+blockClasses[b.class]), nodes)}
		}
		for _, n := range nodes {
			if err := html.Render(&sb, n); err != nil {
				return "", err
			}
		}
	}
	return template.HTML(sb.String()), nil
}

func parseFragment(s string) ([]*html.Node, error) {
	return html.ParseFragment(strings.NewReader(s), &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body})
}

type differ struct {
	opts Options
	dmp  *diffmatchpatch.DiffMatchPatch
	keys map[*html.Node]string

	deadline time.Time
}

type unit struct {
	nodes []*html.Node
	key   string
}

func (u unit) inline() bool { return !isBlock(u.nodes[0]) }

type block struct {
	nodes []*html.Node
	class string // empty if unchanged
}

func (d *differ) units(nodes []*html.Node) (ret []unit) {
	var run []*html.Node
	flush := func() {
		if !isBlank(run) {
			ret = append(ret, unit{nodes: run, key: strings.Trim(d.keyOf(run...), htmlSpace)})
		}
		run = nil
	}
	for _, n := range nodes {
		switch {
		case n.Type == html.CommentNode:
		case isBlock(n):
			flush()
			ret = append(ret, unit{nodes: []*html.Node{n}, key: d.keyOf(n)})
		default:
			run = append(run, n)
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
				ret = append(ret, removedBlock(removed[r]))
			}
			for ; a < addedEnd; a++ {
				ret = append(ret, block{nodes: cloneAll(added[a].nodes), class: ClassAdded})
			}
		}
		for _, p := range d.pairUnits(removed, added, byKind) {
			emitUntil(p[0], p[1])
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
				ret = append(ret, block{nodes: cloneAll(newUnits[newIdx].nodes)})
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
		for i, r := range removed {
			if j := slices.IndexFunc(added[next:], func(a unit) bool { return sameKind(a.nodes[0], r.nodes[0]) }); j >= 0 {
				pairs, next = append(pairs, [2]int{i, next + j}), next+j+1
			}
		}
		return pairs
	}
	type sharedWords struct {
		pair  [2]int
		count int
	}
	var shared []sharedWords // in order on both sides, like the word alignment
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
			shared = append(shared, sharedWords{pair: pair, count: 1})
		}
	})
	bestCount := 0
	for _, s := range shared {
		last := len(pairs) - 1
		switch {
		case last >= 0 && s.pair[0] == pairs[last][0]:
			if s.count > bestCount {
				pairs[last][1], bestCount = s.pair[1], s.count
			}
		case last < 0 || s.pair[1] > pairs[last][1]:
			pairs, bestCount = append(pairs, s.pair), s.count
		}
	}
	return pairs
}

func (d *differ) contents(units []unit) (keys []string, owners []int) {
	for i, u := range units {
		for _, t := range d.tokenize(u.nodes, false) {
			if isContent(&t) {
				keys, owners = append(keys, t.identity), append(owners, i)
			}
		}
	}
	return keys, owners
}

func (d *differ) diffUnit(oldUnit, newUnit unit) []block {
	replace := func() []block {
		return []block{removedBlock(oldUnit), {nodes: cloneAll(newUnit.nodes), class: ClassAdded}}
	}
	if oldUnit.key == newUnit.key {
		return []block{{nodes: cloneAll(newUnit.nodes)}}
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
			nodes = []*html.Node{appendChildren(cloneNode(newNode, false), nodes)}
		}
		return []block{{nodes: nodes, class: util.Iif(changed, ClassChanged, "")}}
	}

	if !sameKind(oldNode, newNode) || isAtomic(oldNode) || isAtomic(newNode) || newNode.DataAtom == atom.Table && columns(oldNode) != columns(newNode) {
		return replace()
	}
	ret := cloneNode(newNode, false)
	changed := d.startTagKey(oldNode) != d.startTagKey(newNode)
	if changed {
		addClass(ret, ClassChanged)
	}
	oldChildren, newChildren := childNodes(oldNode), childNodes(newNode)
	switch {
	case slices.ContainsFunc(oldChildren, isBlock) || slices.ContainsFunc(newChildren, isBlock):
		byKind := newNode.DataAtom == atom.Table || newNode.DataAtom == atom.Tr
		for _, b := range d.diffBlocks(oldChildren, newChildren, byKind) {
			changed = changed || b.class != ""
			appendChildren(ret, markBlock(b))
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
	return []block{{nodes: []*html.Node{ret}, class: util.Iif(changed, ClassChanged, "")}}
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
	identity string // what is compared regardless of the formatting
	full     string // the identity including attributes of an atomic element
}

// key includes the formatting, computed when compared to keep the tokenizer linear in nesting depth
func (t token) key() string { return strings.Join(t.sigs, "") + t.full }

var (
	wordRegexp       = regexp.MustCompile(`(?s)[\p{L}\p{N}\p{M}_]*\p{N}(?:[.,]\p{N}+)+|[\p{Han}\p{Hiragana}\p{Katakana}]|[\p{L}\p{N}\p{M}_]+|\s+|.`)
	whitespaceRegexp = regexp.MustCompile(`\s+`)
)

func (d *differ) tokenize(nodes []*html.Node, lines bool) (ret []token) {
	var walk func(n *html.Node, chain []*html.Node, sigs []string)
	walk = func(n *html.Node, chain []*html.Node, sigs []string) {
		switch {
		case n.Type == html.TextNode:
			parts := wordRegexp.FindAllString(n.Data, -1)
			if lines {
				parts = slices.Collect(strings.Lines(n.Data))
			}
			for _, w := range parts {
				identity := "\x00" + util.Iif(!lines && isSpace(w), " ", w)
				ret = append(ret, token{text: w, chain: chain, sigs: sigs, identity: identity, full: identity})
			}
		case n.Type != html.ElementNode:
		case isAtomic(n) || n.FirstChild == nil:
			ret = append(ret, token{atomic: n, chain: chain, sigs: sigs, identity: "\x01" + d.atomicIdentity(n), full: "\x01" + d.keyOf(n)})
		default:
			chain, sigs = append(slices.Clip(chain), n), append(slices.Clip(sigs), d.startTagKey(n))
			for c := range n.ChildNodes() {
				walk(c, chain, sigs)
			}
		}
	}
	for _, n := range nodes {
		walk(n, nil, nil)
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
				b.add(nil, added[newIdx], ClassChanged)
			case diffmatchpatch.DiffDelete:
				b.add(removed[oldIdx], nil, ClassRemoved)
			case diffmatchpatch.DiffInsert:
				b.add(nil, added[newIdx], ClassAdded)
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
	isSpaceToken := func(t token) bool { return t.atomic == nil && isSpace(t.text) }
	for len(tokens) > 0 && isSpaceToken(tokens[0]) {
		tokens = tokens[1:]
	}
	for len(tokens) > 0 && isSpaceToken(tokens[len(tokens)-1]) {
		tokens = tokens[:len(tokens)-1]
	}
	return tokens
}

func isContent(t *token) bool {
	return t.atomic != nil || isWord(t.text)
}

func (d *differ) diffLines(oldNodes, newNodes []*html.Node) (nodes []*html.Node, changed bool) {
	oldLines, newLines := splitLines(d.tokenize(oldNodes, true)), splitLines(d.tokenize(newNodes, true))
	b := newInlineBuilder()
	addLine := func(line []token, class string) {
		for i := range line {
			if class == ClassRemoved {
				b.add(&line[i], nil, class)
			} else {
				b.add(nil, &line[i], class)
			}
		}
	}
	d.align(keysOf(oldLines, lineKey), keysOf(newLines, lineKey), false, func(typ diffmatchpatch.Operation, oldIdx, newIdx int) {
		switch typ {
		case diffmatchpatch.DiffEqual:
			addLine(newLines[newIdx], "")
		case diffmatchpatch.DiffDelete:
			changed = true
			addLine(oldLines[oldIdx], ClassRemoved)
		case diffmatchpatch.DiffInsert:
			changed = true
			addLine(newLines[newIdx], ClassAdded)
		}
	})
	return b.finish(), changed
}

func splitLines(tokens []token) (lines [][]token) {
	start := 0
	for i, t := range tokens {
		if strings.HasSuffix(t.text, "\n") || i == len(tokens)-1 {
			lines, start = append(lines, tokens[start:i+1]), i+1
		}
	}
	return lines
}

func lineKey(line []token) string {
	var sb strings.Builder
	for _, t := range line {
		sb.WriteString(util.Iif(t.atomic == nil, t.text, t.identity)) // highlighting may split a line differently
	}
	return sb.String()
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
	t := util.IfZero(newToken, oldToken)
	isOpen := func(i int) bool {
		el := b.stack[i]
		for side, st := range sides {
			if st != nil && el.sources[side] != nil && el.sources[side] != st.chain[i] {
				return false
			}
		}
		return el.sig == t.sigs[i]
	}
	common := 0
	for common < len(b.stack) && common < len(t.sigs) && isOpen(common) {
		common++
	}
	b.stack = b.stack[:common]
	for i := common; i < len(t.chain); i++ {
		el := cloneNode(t.chain[i], false)
		if newToken == nil {
			removeIDs(el)
		}
		b.parent().AppendChild(el)
		b.stack = append(b.stack, openElement{node: el, sig: t.sigs[i]})
	}
	for i := range b.stack {
		if i < common && newToken != nil && b.stack[i].sources[1] == nil {
			b.stack[i].node.Attr = slices.Clone(newToken.chain[i].Attr) // opened by a removed token without its id
		}
		for side, st := range sides {
			if st != nil {
				b.stack[i].sources[side] = st.chain[i]
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
	if t.atomic != nil {
		el := cloneNode(t.atomic, true)
		if newToken == nil {
			removeIDs(el)
		}
		target.AppendChild(el)
		return
	}
	if b.text == nil || target.LastChild != b.text {
		b.flushText()
		b.text = &html.Node{Type: html.TextNode}
		target.AppendChild(b.text)
	}
	b.textBuf.WriteString(t.text)
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
	for _, c := range ret {
		b.root.RemoveChild(c)
	}
	return ret
}

// align diffs key sequences mapped to runes and calls fn for each item
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
		for i, k := range keys {
			r, ok := runeOf[k]
			if !ok {
				r = rune(len(runeOf) + utf8.RuneSelf) // ASCII would make the semantic cleanup see word boundaries
				if r >= 0xD800 {
					r += 0x800 // skip the surrogates, they can't be encoded in the diff texts
				}
				runeOf[k] = r
			}
			ret[i] = r
		}
		return ret
	}
	diffs := d.dmp.DiffMainRunes(toRunes(oldKeys), toRunes(newKeys), false)
	if cleanup {
		diffs = d.dmp.DiffCleanupSemantic(diffs)
	}
	for _, df := range diffs {
		emit(df.Type, utf8.RuneCountInString(df.Text))
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

func removedBlock(u unit) block {
	nodes := cloneAll(u.nodes)
	for _, n := range nodes {
		removeIDs(n) // the added copy keeps them, anchors must not jump to the removed one
	}
	return block{nodes: nodes, class: ClassRemoved}
}

func markBlock(b block) []*html.Node {
	if b.class == "" || b.class == ClassChanged {
		return b.nodes
	}
	if !isBlock(b.nodes[0]) {
		return []*html.Node{appendChildren(newMark(b.class), b.nodes)}
	}
	if isAtomic(b.nodes[0]) {
		// client-side renderers (math, mermaid) replace the element, so the class must be on a wrapper
		return []*html.Node{appendChildren(newElement(atom.Div, b.class), b.nodes)}
	}
	addClass(b.nodes[0], b.class)
	return b.nodes
}

func (d *differ) keyOf(nodes ...*html.Node) string {
	if len(nodes) == 1 {
		return d.nodeKey(nodes[0])
	}
	var sb strings.Builder
	for _, n := range nodes {
		sb.WriteString(d.nodeKey(n))
	}
	return sb.String()
}

// nodeKey serializes a node once, the keys of its descendants are substrings of it
func (d *differ) nodeKey(node *html.Node) string {
	if key, ok := d.keys[node]; ok {
		return key
	}
	type span struct {
		node       *html.Node
		start, end int
	}
	var sb strings.Builder
	var spans []span
	var write func(n *html.Node)
	write = func(n *html.Node) {
		start := sb.Len()
		switch n.Type {
		case html.TextNode:
			sb.WriteString(html.EscapeString(normalizeSpace(n)))
		case html.ElementNode:
			sb.WriteString(d.startTagKey(n))
			for c := range n.ChildNodes() {
				write(c)
			}
			sb.WriteString("</" + n.Data + ">")
		}
		spans = append(spans, span{n, start, sb.Len()})
	}
	write(node)
	key := sb.String()
	for _, sp := range spans {
		d.keys[sp.node] = key[sp.start:sp.end]
	}
	return key
}

func (d *differ) startTagKey(n *html.Node) string {
	var sb strings.Builder
	sb.WriteString("<" + n.Data)
	for _, a := range n.Attr {
		if a.Key != "id" && a.Key != "data-source-position" { // ids follow the content
			sb.WriteString(" " + a.Key + `="` + html.EscapeString(d.attrKey(a)) + `"`)
		}
	}
	sb.WriteString(">")
	return sb.String()
}

func (d *differ) attrKey(a html.Attribute) string {
	if (a.Key == "href" || a.Key == "src" || a.Key == "srcset") && d.opts.NormalizeLink != nil {
		return d.opts.NormalizeLink(a.Val)
	}
	return a.Val
}

// identityAttrs tell elements apart, other attribute changes like "checked" are edits in place
var identityAttrs = map[atom.Atom]string{atom.Img: "src", atom.Input: "type"}

func (d *differ) atomicIdentity(n *html.Node) string {
	if key, ok := identityAttrs[n.DataAtom]; ok {
		return n.Data + " " + d.attrKey(html.Attribute{Key: key, Val: attrValue(n, key)})
	}
	return d.keyOf(n)
}

// whitespace runs render alike outside preformatted text
func normalizeSpace(textNode *html.Node) string {
	for n := textNode.Parent; n != nil; n = n.Parent {
		if n.DataAtom == atom.Pre {
			return textNode.Data
		}
	}
	return whitespaceRegexp.ReplaceAllString(textNode.Data, " ")
}

func removeIDs(n *html.Node) {
	n.Attr = slices.DeleteFunc(n.Attr, func(a html.Attribute) bool { return a.Key == "id" })
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		removeIDs(c)
	}
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

func isBlock(n *html.Node) bool {
	return n.Type == html.ElementNode && blockAtoms[n.DataAtom]
}

func sameKind(a, b *html.Node) bool {
	return a.Data == b.Data || isHeading(a) && isHeading(b) || isCell(a) && isCell(b)
}

func isHeading(n *html.Node) bool {
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		return true
	}
	return false
}

func columns(table *html.Node) int {
	for n := range table.Descendants() {
		if n.DataAtom == atom.Tr {
			return len(slices.DeleteFunc(childNodes(n), func(c *html.Node) bool { return !isCell(c) }))
		}
	}
	return 0
}

func isCell(n *html.Node) bool {
	return n.DataAtom == atom.Td || n.DataAtom == atom.Th
}

func isAtomic(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	isInlineCode := n.DataAtom == atom.Code && (n.Parent == nil || n.Parent.DataAtom != atom.Pre)
	return atomicAtoms[n.DataAtom] || isInlineCode || isClientRenderedCode(n)
}

// isClientRenderedCode reports math and mermaid code, mixed old and new source would render wrong
func isClientRenderedCode(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Code:
		classes := strings.Fields(attrValue(n, "class"))
		return slices.Contains(classes, "language-math") || slices.Contains(classes, "language-mermaid")
	case atom.Pre, atom.Div:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if isClientRenderedCode(c) {
				return true
			}
		}
	}
	return false
}

const htmlSpace = " \t\n\f\r"

func isSpace(s string) bool {
	return strings.Trim(s, htmlSpace) == ""
}

func isWord(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

func isBlank(nodes []*html.Node) bool {
	for _, n := range nodes {
		if n.Type != html.TextNode || !isSpace(n.Data) {
			return false
		}
	}
	return true
}

func childNodes(n *html.Node) []*html.Node {
	return slices.Collect(n.ChildNodes())
}

func appendChildren(n *html.Node, children []*html.Node) *html.Node {
	for _, c := range children {
		n.AppendChild(c)
	}
	return n
}

func cloneNode(n *html.Node, deep bool) *html.Node {
	ret := &html.Node{Type: n.Type, DataAtom: n.DataAtom, Data: n.Data, Namespace: n.Namespace, Attr: slices.Clone(n.Attr)}
	if deep {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			ret.AppendChild(cloneNode(c, true))
		}
	}
	return ret
}

func cloneAll(nodes []*html.Node) []*html.Node {
	ret := make([]*html.Node, len(nodes))
	for i, n := range nodes {
		ret[i] = cloneNode(n, true)
	}
	return ret
}

func newMark(class string) *html.Node {
	switch class {
	case ClassAdded:
		return newElement(atom.Ins, class)
	case ClassRemoved:
		return newElement(atom.Del, class)
	}
	return newElement(atom.Span, class)
}

func newElement(a atom.Atom, class string) *html.Node {
	return &html.Node{Type: html.ElementNode, DataAtom: a, Data: a.String(), Attr: []html.Attribute{{Key: "class", Val: class}}}
}

func addClass(n *html.Node, class string) {
	for i, a := range n.Attr {
		if a.Key == "class" {
			n.Attr[i].Val = strings.TrimSpace(a.Val + " " + class)
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: "class", Val: class})
}

func attrValue(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
