// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/net/html"
)

// HTMLDiff produces a rich inline diff between two rendered HTML fragments.
// Inserted runs are wrapped in <span class="added-code">, deleted runs in
// <span class="removed-code">. It is intended for rendered content such as
// the rich diff view where both the old and new output should be visible
// simultaneously.
//
// Two passes: first a block-level diff of the top-level elements in the
// fragment (paragraphs, headings, lists, code blocks, ...) so that structural
// changes — reordered paragraphs, a paragraph replaced by a list, etc. — do
// not produce a wall of red/green. Then, for each block that was changed in
// place, a word-level HTML diff runs within that block via
// highlightCodeDiff.diffHTMLWithHighlight. If fragment parsing fails we fall
// back to the single-pass word-level diff over the whole document.
func HTMLDiff(oldHTML, newHTML template.HTML) template.HTML {
	oldBlocks, okOld := splitTopLevelBlocks(string(oldHTML))
	newBlocks, okNew := splitTopLevelBlocks(string(newHTML))
	if !okOld || !okNew {
		return htmlDiffWordLevel(oldHTML, newHTML)
	}
	return renderBlockDiff(oldBlocks, newBlocks)
}

// htmlDiffWordLevel runs the existing placeholder-based word-level diff over
// a single HTML fragment. It is the inner pass used for changed blocks, and
// the fallback when block-level splitting fails.
func htmlDiffWordLevel(oldHTML, newHTML template.HTML) template.HTML {
	hcd := newHighlightCodeDiff()
	return hcd.diffHTMLWithHighlight(oldHTML, newHTML)
}

// topLevelBlock is a single top-level element from a parsed HTML fragment,
// carrying both the serialized form (used for the block-level diff) and the
// root tag name (used to decide whether two adjacent blocks should be paired
// for a word-level inner diff).
type topLevelBlock struct {
	html string
	tag  string
}

// splitTopLevelBlocks parses an HTML fragment and returns the serialized form
// of each top-level child node (i.e. the children of <body> after wrapping).
// Whitespace-only text nodes between blocks are dropped; the diff would treat
// them as churn otherwise.
func splitTopLevelBlocks(fragment string) ([]topLevelBlock, bool) {
	if fragment == "" {
		return nil, true
	}
	node, err := html.Parse(strings.NewReader("<html><body>" + fragment + "</body></html>"))
	if err != nil {
		return nil, false
	}
	// walk to <body>
	var body *html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if body != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
	if body == nil {
		return nil, false
	}

	var blocks []topLevelBlock
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
			continue
		}
		var buf bytes.Buffer
		if err := html.Render(&buf, c); err != nil {
			return nil, false
		}
		tag := ""
		if c.Type == html.ElementNode {
			tag = c.Data
		}
		blocks = append(blocks, topLevelBlock{html: buf.String(), tag: tag})
	}
	return blocks, true
}

// renderBlockDiff computes a block-level diff between two sequences of
// top-level blocks. Each unique block string is mapped to a single rune so
// diffmatchpatch.DiffMain can operate over blocks rather than characters,
// exactly as diffmatchpatch's DiffLinesToChars pattern works for line-mode
// diffs. Equal runs are emitted verbatim; adjacent delete+insert runs are
// paired up and each pair is rendered as a word-level diff so in-place edits
// inside a paragraph still highlight just the changed words. Remaining
// unmatched blocks are wrapped whole in added-code/removed-code spans.
func renderBlockDiff(oldBlocks, newBlocks []topLevelBlock) template.HTML {
	blockToRune := map[string]rune{}
	runeToBlock := map[rune]topLevelBlock{}
	// Start well above ASCII and outside the private-use ranges that the
	// word-level pass already uses for placeholder tokens.
	next := rune(0xE000)
	encode := func(blocks []topLevelBlock) string {
		var sb strings.Builder
		for _, b := range blocks {
			r, ok := blockToRune[b.html]
			if !ok {
				r = next
				next++
				blockToRune[b.html] = r
				runeToBlock[r] = b
			}
			sb.WriteRune(r)
		}
		return sb.String()
	}
	encodedOld := encode(oldBlocks)
	encodedNew := encode(newBlocks)

	dmp := defaultDiffMatchPatch()
	diffs := dmp.DiffMain(encodedOld, encodedNew, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	wrapRemoved := func(sb *strings.Builder, block topLevelBlock) {
		sb.WriteString(`<span class="removed-code">`)
		sb.WriteString(block.html)
		sb.WriteString(`</span>`)
	}
	wrapAdded := func(sb *strings.Builder, block topLevelBlock) {
		sb.WriteString(`<span class="added-code">`)
		sb.WriteString(block.html)
		sb.WriteString(`</span>`)
	}

	var out strings.Builder
	for i := 0; i < len(diffs); i++ {
		d := diffs[i]
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			for _, r := range d.Text {
				out.WriteString(runeToBlock[r].html)
			}
		case diffmatchpatch.DiffDelete:
			delRunes := []rune(d.Text)
			// If an insert immediately follows, pair blocks one-to-one so in-place
			// edits show up as inline changes rather than whole-block replacements.
			// Only pair when both sides share a root tag — a <p>/<ul> pairing would
			// produce broken nesting when we word-diff their raw HTML.
			var insRunes []rune
			if i+1 < len(diffs) && diffs[i+1].Type == diffmatchpatch.DiffInsert {
				insRunes = []rune(diffs[i+1].Text)
				i++
			}
			paired := 0
			for paired < len(delRunes) && paired < len(insRunes) {
				oldBlock := runeToBlock[delRunes[paired]]
				newBlock := runeToBlock[insRunes[paired]]
				if oldBlock.tag == "" || oldBlock.tag != newBlock.tag {
					break
				}
				out.WriteString(string(htmlDiffWordLevel(template.HTML(oldBlock.html), template.HTML(newBlock.html))))
				paired++
			}
			for j := paired; j < len(delRunes); j++ {
				wrapRemoved(&out, runeToBlock[delRunes[j]])
			}
			for j := paired; j < len(insRunes); j++ {
				wrapAdded(&out, runeToBlock[insRunes[j]])
			}
		case diffmatchpatch.DiffInsert:
			for _, r := range d.Text {
				wrapAdded(&out, runeToBlock[r])
			}
		}
	}
	return template.HTML(out.String())
}
