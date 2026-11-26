// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package htmldiff marks the changes between two rendered HTML documents in the structure of the new one:
// blocks are aligned first, then changed blocks are compared word by word.
package htmldiff

import (
	"html/template"
	"regexp"
	"slices"
	"strings"
	"time"
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

	diffTimeout = 2 * time.Second // for a whole document
)

type Options struct {
	// NormalizeLink rewrites "href" and "src" values before comparing, e.g. to ignore the commit ID in repository links
	NormalizeLink func(link string) string
}

// Diff returns newHTML with the changes from oldHTML marked by the ClassAdded, ClassRemoved and ClassChanged classes.
// The inputs must be sanitized already, the result only adds "ins", "del" and "div" elements and "class" attributes.
func Diff(oldHTML, newHTML string, opts Options) (template.HTML, error) {
	oldNodes, err := parseFragment(oldHTML)
	if err != nil {
		return "", err
	}
	newNodes, err := parseFragment(newHTML)
	if err != nil {
		return "", err
	}
	d := &differ{opts: opts, dmp: diffmatchpatch.New(), keys: map[*html.Node]string{}, deadline: time.Now().Add(diffTimeout)}

	var sb strings.Builder
	for _, n := range d.diffBlocks(oldNodes, newNodes) {
		if err := html.Render(&sb, n); err != nil {
			return "", err
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

// unit is a block element, or a run of inline nodes between blocks
type unit struct {
	nodes  []*html.Node
	inline bool
	key    string
}

func (d *differ) units(nodes []*html.Node) (ret []unit) {
	var run []*html.Node
	flush := func() {
		if !isBlank(run) {
			ret = append(ret, unit{nodes: run, inline: true, key: d.keyOf(run...)})
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

func (d *differ) diffBlocks(oldNodes, newNodes []*html.Node) (ret []*html.Node) {
	oldUnits, newUnits := d.units(oldNodes), d.units(newNodes)
	var removed, added []unit
	flush := func() {
		paired := min(len(removed), len(added))
		for i := range paired {
			ret = append(ret, d.diffUnit(removed[i], added[i])...)
		}
		for _, u := range removed[paired:] {
			ret = append(ret, markUnit(u, ClassRemoved)...)
		}
		for _, u := range added[paired:] {
			ret = append(ret, markUnit(u, ClassAdded)...)
		}
		removed, added = nil, nil
	}
	d.align(keysOf(oldUnits, unit.getKey), keysOf(newUnits, unit.getKey), false, func(typ diffmatchpatch.Operation, oldIdx, newIdx int) {
		switch typ {
		case diffmatchpatch.DiffEqual:
			flush()
			ret = append(ret, cloneAll(newUnits[newIdx].nodes)...)
		case diffmatchpatch.DiffDelete:
			removed = append(removed, oldUnits[oldIdx])
		case diffmatchpatch.DiffInsert:
			added = append(added, newUnits[newIdx])
		}
	})
	flush()
	return ret
}

func (d *differ) diffUnit(oldUnit, newUnit unit) []*html.Node {
	replace := func() []*html.Node {
		return append(markUnit(oldUnit, ClassRemoved), markUnit(newUnit, ClassAdded)...)
	}
	if oldUnit.inline != newUnit.inline {
		return replace()
	}
	if oldUnit.inline {
		if nodes, ok := d.diffInline(oldUnit.nodes, newUnit.nodes); ok {
			return nodes
		}
		return replace()
	}

	oldNode, newNode := oldUnit.nodes[0], newUnit.nodes[0]
	if oldNode.Data != newNode.Data || isAtomic(oldNode) || isAtomic(newNode) {
		return replace()
	}
	ret := cloneNode(newNode, false)
	if !sameAttrs(oldNode, newNode, blockAttrsThatCount) {
		addClass(ret, ClassChanged)
	}
	var children []*html.Node
	if hasBlockChild(oldNode) || hasBlockChild(newNode) {
		children = d.diffBlocks(childNodes(oldNode), childNodes(newNode))
	} else {
		var ok bool
		children, ok = d.diffInline(childNodes(oldNode), childNodes(newNode))
		isCell := newNode.DataAtom == atom.Td || newNode.DataAtom == atom.Th
		if !ok && !isCell { // a replaced cell would add a column
			return replace()
		}
	}
	for _, c := range children {
		ret.AppendChild(c)
	}
	return []*html.Node{ret}
}

// token is a word, a whitespace run, a symbol, or an atomic inline element like an image
type token struct {
	text   string
	atomic *html.Node
	chain  []*html.Node // inline ancestors, outermost first
	sigs   []string
	key    string
}

var wordRegexp = regexp.MustCompile(`(?s)[\p{Han}\p{Hiragana}\p{Katakana}]|[\p{L}\p{N}\p{M}_]+|\s+|.`)

func (d *differ) tokenize(nodes []*html.Node) (ret []token) {
	var walk func(n *html.Node, chain []*html.Node, sigs []string)
	walk = func(n *html.Node, chain []*html.Node, sigs []string) {
		prefix := strings.Join(sigs, "")
		switch {
		case n.Type == html.TextNode:
			for _, w := range wordRegexp.FindAllString(n.Data, -1) {
				ret = append(ret, token{text: w, chain: chain, sigs: sigs, key: prefix + "\x00" + w})
			}
		case n.Type != html.ElementNode:
		case isAtomic(n) || n.FirstChild == nil:
			ret = append(ret, token{atomic: n, chain: chain, sigs: sigs, key: prefix + "\x01" + d.keyOf(n)})
		default:
			chain, sigs = append(slices.Clip(chain), n), append(slices.Clip(sigs), d.startTagKey(n))
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, chain, sigs)
			}
		}
	}
	for _, n := range nodes {
		walk(n, nil, nil)
	}
	return ret
}

// diffInline returns false if the old and new nodes have no words in common, then a whole replacement reads better
func (d *differ) diffInline(oldNodes, newNodes []*html.Node) ([]*html.Node, bool) {
	oldTokens, newTokens := d.tokenize(oldNodes), d.tokenize(newNodes)
	b := &inlineBuilder{root: &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}}
	hasCommon := false
	d.align(keysOf(oldTokens, token.getKey), keysOf(newTokens, token.getKey), true, func(typ diffmatchpatch.Operation, oldIdx, newIdx int) {
		switch typ {
		case diffmatchpatch.DiffEqual:
			t := &newTokens[newIdx]
			hasCommon = hasCommon || t.atomic != nil || strings.TrimSpace(t.text) != ""
			b.add(&oldTokens[oldIdx], t, "")
		case diffmatchpatch.DiffDelete:
			b.add(&oldTokens[oldIdx], nil, ClassRemoved)
		case diffmatchpatch.DiffInsert:
			b.add(nil, &newTokens[newIdx], ClassAdded)
		}
	})
	b.flushText()
	return detachChildren(b.root), hasCommon
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
	sources [2]*html.Node // the old and new element it stands for, equal siblings must not be merged
}

// add appends a token, oldToken is nil for an added one and newToken is nil for a removed one
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
		if newToken != nil && b.stack[i].sources[1] == nil {
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

// align diffs two key sequences by mapping each distinct key to a rune, and calls fn for each item with its index
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
	if d.dmp.DiffTimeout <= 0 { // out of time, fall back to a whole replacement
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
				r = rune(len(runeOf) + 1)
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

func (t token) getKey() string { return t.key }

func markUnit(u unit, class string) []*html.Node {
	if u.inline {
		el := newMark(class)
		for _, n := range cloneAll(u.nodes) {
			if class == ClassRemoved {
				removeIDs(n)
			}
			el.AppendChild(n)
		}
		return []*html.Node{el}
	}
	n := cloneNode(u.nodes[0], true)
	if class == ClassRemoved {
		removeIDs(n) // the added copy keeps them, anchors must not jump to the removed one
	}
	if isAtomic(n) {
		// client-side renderers (math, mermaid) replace the element, so the class must be on a wrapper
		el := newElement(atom.Div, class)
		el.AppendChild(n)
		return []*html.Node{el}
	}
	addClass(n, class)
	return []*html.Node{n}
}

// keyOf serializes nodes for comparison, ignoring attributes that change without the content changing
func (d *differ) keyOf(nodes ...*html.Node) string {
	var sb strings.Builder
	for _, n := range nodes {
		if k, ok := d.keys[n]; ok {
			sb.WriteString(k)
			continue
		}
		var nsb strings.Builder
		switch n.Type {
		case html.TextNode:
			nsb.WriteString(html.EscapeString(n.Data))
		case html.ElementNode:
			nsb.WriteString(d.startTagKey(n))
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				nsb.WriteString(d.keyOf(c))
			}
			nsb.WriteString("</" + n.Data + ">")
		}
		d.keys[n] = nsb.String()
		sb.WriteString(d.keys[n])
	}
	return sb.String()
}

func (d *differ) startTagKey(n *html.Node) string {
	var sb strings.Builder
	sb.WriteString("<" + n.Data)
	for _, a := range n.Attr {
		val := a.Val
		switch a.Key {
		case "data-source-position":
			continue
		case "href", "src":
			if d.opts.NormalizeLink != nil {
				val = d.opts.NormalizeLink(val)
			}
		}
		sb.WriteString(" " + a.Key + "=" + html.EscapeString(val))
	}
	sb.WriteString(">")
	return sb.String()
}

// blockAttrsThatCount change how a block looks, others like a heading's "id" follow its content
var blockAttrsThatCount = []string{"align", "colspan", "reversed", "rowspan", "start", "type"}

func sameAttrs(a, b *html.Node, keys []string) bool {
	for _, k := range keys {
		if attrValue(a, k) != attrValue(b, k) {
			return false
		}
	}
	return true
}

func removeIDs(n *html.Node) {
	n.Attr = slices.DeleteFunc(n.Attr, func(a html.Attribute) bool { return a.Key == "id" })
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		removeIDs(c)
	}
}

var blockAtoms = map[atom.Atom]bool{
	atom.Address: true, atom.Article: true, atom.Aside: true, atom.Blockquote: true, atom.Caption: true,
	atom.Dd: true, atom.Details: true, atom.Div: true, atom.Dl: true, atom.Dt: true, atom.Figcaption: true,
	atom.Figure: true, atom.Footer: true, atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true,
	atom.H5: true, atom.H6: true, atom.Header: true, atom.Hr: true, atom.Li: true, atom.Ol: true,
	atom.P: true, atom.Pre: true, atom.Section: true, atom.Table: true, atom.Tbody: true, atom.Td: true,
	atom.Tfoot: true, atom.Th: true, atom.Thead: true, atom.Tr: true, atom.Ul: true,
}

// atomicAtoms are never merged inside: they are shown as removed and added as a whole
var atomicAtoms = map[atom.Atom]bool{
	atom.Audio: true, atom.Br: true, atom.Canvas: true, atom.Details: true, atom.Embed: true, atom.Hr: true,
	atom.Iframe: true, atom.Img: true, atom.Input: true, atom.Math: true, atom.Object: true, atom.Picture: true,
	atom.Svg: true, atom.Video: true,
}

func isBlock(n *html.Node) bool {
	return n.Type == html.ElementNode && blockAtoms[n.DataAtom]
}

func isAtomic(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	return atomicAtoms[n.DataAtom] || isClientRenderedCode(n)
}

// isClientRenderedCode reports code that the frontend renders (math, mermaid) and its wrapping blocks,
// mixing old and new source would render wrong content
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

func hasBlockChild(n *html.Node) bool {
	return slices.ContainsFunc(childNodes(n), isBlock)
}

func isBlank(nodes []*html.Node) bool {
	for _, n := range nodes {
		if n.Type != html.TextNode || strings.TrimSpace(n.Data) != "" {
			return false
		}
	}
	return true
}

func childNodes(n *html.Node) []*html.Node {
	return slices.Collect(n.ChildNodes())
}

func detachChildren(n *html.Node) []*html.Node {
	ret := childNodes(n)
	for _, c := range ret {
		n.RemoveChild(c)
	}
	return ret
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
	return newElement(util.Iif(class == ClassAdded, atom.Ins, atom.Del), class)
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
