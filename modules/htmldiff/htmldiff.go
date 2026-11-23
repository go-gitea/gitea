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
)

type Options struct {
	// NormalizeLink rewrites "href" and "src" values before comparing, e.g. to ignore the commit ID in repository links
	NormalizeLink func(link string) string
}

// Diff returns newHTML with the changes from oldHTML marked by the ClassAdded and ClassRemoved classes.
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
	d := &differ{opts: opts, dmp: diffmatchpatch.New(), keys: map[*html.Node]string{}}
	d.dmp.DiffTimeout = time.Second

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
	oldIdx, newIdx := 0, 0
	for _, op := range d.align(unitKeys(oldUnits), unitKeys(newUnits), false) {
		for range op.count {
			switch op.typ {
			case diffmatchpatch.DiffEqual:
				flush()
				ret = append(ret, cloneAll(newUnits[newIdx].nodes)...)
				oldIdx++
				newIdx++
			case diffmatchpatch.DiffDelete:
				removed = append(removed, oldUnits[oldIdx])
				oldIdx++
			case diffmatchpatch.DiffInsert:
				added = append(added, newUnits[newIdx])
				newIdx++
			}
		}
	}
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
	oldIdx, newIdx := 0, 0
	for _, op := range d.align(tokenKeys(oldTokens), tokenKeys(newTokens), true) {
		for range op.count {
			switch op.typ {
			case diffmatchpatch.DiffEqual:
				t := newTokens[newIdx]
				hasCommon = hasCommon || t.atomic != nil || strings.TrimSpace(t.text) != ""
				b.add(t, "")
				oldIdx++
				newIdx++
			case diffmatchpatch.DiffDelete:
				b.add(oldTokens[oldIdx], ClassRemoved)
				oldIdx++
			case diffmatchpatch.DiffInsert:
				b.add(newTokens[newIdx], ClassAdded)
				newIdx++
			}
		}
	}
	return detachChildren(b.root), hasCommon
}

type inlineBuilder struct {
	root      *html.Node
	stack     []*html.Node
	stackSigs []string
	mark      *html.Node
	markClass string
}

func (b *inlineBuilder) add(t token, class string) {
	common := 0
	for common < len(b.stackSigs) && common < len(t.sigs) && b.stackSigs[common] == t.sigs[common] {
		common++
	}
	b.stack, b.stackSigs = b.stack[:common], b.stackSigs[:common]
	for i := common; i < len(t.chain); i++ {
		el := cloneNode(t.chain[i], false)
		b.parent().AppendChild(el)
		b.stack, b.stackSigs = append(b.stack, el), append(b.stackSigs, t.sigs[i])
	}

	target := b.parent()
	if class != "" {
		if b.mark == nil || b.mark.Parent != target || target.LastChild != b.mark || b.markClass != class {
			b.mark = newElement(util.Iif(class == ClassAdded, atom.Ins, atom.Del), class)
			b.markClass = class
			target.AppendChild(b.mark)
		}
		target = b.mark
	}
	if t.atomic != nil {
		target.AppendChild(cloneNode(t.atomic, true))
	} else if target.LastChild != nil && target.LastChild.Type == html.TextNode {
		target.LastChild.Data += t.text
	} else {
		target.AppendChild(&html.Node{Type: html.TextNode, Data: t.text})
	}
}

func (b *inlineBuilder) parent() *html.Node {
	if len(b.stack) == 0 {
		return b.root
	}
	return b.stack[len(b.stack)-1]
}

type alignOp struct {
	typ   diffmatchpatch.Operation
	count int
}

// align diffs two key sequences by mapping each distinct key to a rune
func (d *differ) align(oldKeys, newKeys []string, cleanup bool) []alignOp {
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
	ret := make([]alignOp, 0, len(diffs))
	for _, df := range diffs {
		ret = append(ret, alignOp{typ: df.Type, count: utf8.RuneCountInString(df.Text)})
	}
	return ret
}

func unitKeys(units []unit) []string {
	ret := make([]string, len(units))
	for i, u := range units {
		ret[i] = u.key
	}
	return ret
}

func tokenKeys(tokens []token) []string {
	ret := make([]string, len(tokens))
	for i, t := range tokens {
		ret[i] = t.key
	}
	return ret
}

func markUnit(u unit, class string) []*html.Node {
	if u.inline {
		el := newElement(util.Iif(class == ClassAdded, atom.Ins, atom.Del), class)
		for _, n := range cloneAll(u.nodes) {
			el.AppendChild(n)
		}
		return []*html.Node{el}
	}
	n := cloneNode(u.nodes[0], true)
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
	if atomicAtoms[n.DataAtom] || isClientRenderedCode(n) {
		return true
	}
	if n.DataAtom == atom.Pre || n.DataAtom == atom.Div {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if isAtomic(c) {
				return true
			}
		}
	}
	return false
}

// isClientRenderedCode reports code that the frontend renders (math, mermaid), mixing old and new source would render wrong content
func isClientRenderedCode(n *html.Node) bool {
	if n.DataAtom != atom.Code {
		return false
	}
	classes := strings.Fields(attrValue(n, "class"))
	return slices.Contains(classes, "language-math") || slices.Contains(classes, "language-mermaid")
}

func hasBlockChild(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if isBlock(c) {
			return true
		}
	}
	return false
}

func isBlank(nodes []*html.Node) bool {
	for _, n := range nodes {
		if n.Type != html.TextNode || strings.TrimSpace(n.Data) != "" {
			return false
		}
	}
	return true
}

func childNodes(n *html.Node) (ret []*html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		ret = append(ret, c)
	}
	return ret
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
