// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mdstripper

import (
	"bytes"

	"github.com/russross/blackfriday"
)

// MarkdownStripper extends blackfriday.Renderer
type MarkdownStripper struct {
	blackfriday.Renderer
	links     []string
	coallesce bool
}

const (
	blackfridayExtensions = 0 |
		blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_NO_EMPTY_LINE_BEFORE_BLOCK |
		blackfriday.EXTENSION_DEFINITION_LISTS |
		blackfriday.EXTENSION_FOOTNOTES |
		blackfriday.EXTENSION_HEADER_IDS |
		blackfriday.EXTENSION_AUTO_HEADER_IDS |
		// Not included in modules/markup/markdown/markdown.go;
		// required here to process inline links
		blackfriday.EXTENSION_AUTOLINK
)

//revive:disable:var-naming Implementing the Rendering interface requires breaking some linting rules

// StripMarkdown parses markdown content by removing all markup and code blocks
//	in order to extract links and other references
func StripMarkdown(rawBytes []byte) (string, []string) {
	stripper := &MarkdownStripper{
		links: make([]string, 0, 10),
	}
	body := blackfriday.Markdown(rawBytes, stripper, blackfridayExtensions)
	return string(body), stripper.GetLinks()
}

// StripMarkdownBytes parses markdown content by removing all markup and code blocks
//	in order to extract links and other references
func StripMarkdownBytes(rawBytes []byte) ([]byte, []string) {
	stripper := &MarkdownStripper{
		links: make([]string, 0, 10),
	}
	body := blackfriday.Markdown(rawBytes, stripper, blackfridayExtensions)
	return body, stripper.GetLinks()
}

// block-level callbacks

// BlockCode dummy function to proceed with rendering
func (r *MarkdownStripper) BlockCode(out *bytes.Buffer, text []byte, infoString string) {
	// Not rendered
	r.coallesce = false
}

// BlockQuote dummy function to proceed with rendering
func (r *MarkdownStripper) BlockQuote(out *bytes.Buffer, text []byte) {
	// FIXME: perhaps it's better to leave out block quote for this?
	r.processString(out, text, false)
}

// BlockHtml dummy function to proceed with rendering
func (r *MarkdownStripper) BlockHtml(out *bytes.Buffer, text []byte) { //nolint
	// Not rendered
	r.coallesce = false
}

// Header dummy function to proceed with rendering
func (r *MarkdownStripper) Header(out *bytes.Buffer, text func() bool, level int, id string) {
	text()
	r.coallesce = false
}

// HRule dummy function to proceed with rendering
func (r *MarkdownStripper) HRule(out *bytes.Buffer) {
	// Not rendered
	r.coallesce = false
}

// List dummy function to proceed with rendering
func (r *MarkdownStripper) List(out *bytes.Buffer, text func() bool, flags int) {
	text()
	r.coallesce = false
}

// ListItem dummy function to proceed with rendering
func (r *MarkdownStripper) ListItem(out *bytes.Buffer, text []byte, flags int) {
	r.processString(out, text, false)
}

// Paragraph dummy function to proceed with rendering
func (r *MarkdownStripper) Paragraph(out *bytes.Buffer, text func() bool) {
	text()
	r.coallesce = false
}

// Table dummy function to proceed with rendering
func (r *MarkdownStripper) Table(out *bytes.Buffer, header []byte, body []byte, columnData []int) {
	r.processString(out, header, false)
	r.processString(out, body, false)
}

// TableRow dummy function to proceed with rendering
func (r *MarkdownStripper) TableRow(out *bytes.Buffer, text []byte) {
	r.processString(out, text, false)
}

// TableHeaderCell dummy function to proceed with rendering
func (r *MarkdownStripper) TableHeaderCell(out *bytes.Buffer, text []byte, flags int) {
	r.processString(out, text, false)
}

// TableCell dummy function to proceed with rendering
func (r *MarkdownStripper) TableCell(out *bytes.Buffer, text []byte, flags int) {
	r.processString(out, text, false)
}

// Footnotes dummy function to proceed with rendering
func (r *MarkdownStripper) Footnotes(out *bytes.Buffer, text func() bool) {
	text()
}

// FootnoteItem dummy function to proceed with rendering
func (r *MarkdownStripper) FootnoteItem(out *bytes.Buffer, name, text []byte, flags int) {
	r.processString(out, text, false)
}

// TitleBlock dummy function to proceed with rendering
func (r *MarkdownStripper) TitleBlock(out *bytes.Buffer, text []byte) {
	r.processString(out, text, false)
}

// Span-level callbacks

// AutoLink dummy function to proceed with rendering
func (r *MarkdownStripper) AutoLink(out *bytes.Buffer, link []byte, kind int) {
	r.processLink(out, link, []byte{})
}

// CodeSpan dummy function to proceed with rendering
func (r *MarkdownStripper) CodeSpan(out *bytes.Buffer, text []byte) {
	// Not rendered
	r.coallesce = false
}

// DoubleEmphasis dummy function to proceed with rendering
func (r *MarkdownStripper) DoubleEmphasis(out *bytes.Buffer, text []byte) {
	r.processString(out, text, false)
}

// Emphasis dummy function to proceed with rendering
func (r *MarkdownStripper) Emphasis(out *bytes.Buffer, text []byte) {
	r.processString(out, text, false)
}

// Image dummy function to proceed with rendering
func (r *MarkdownStripper) Image(out *bytes.Buffer, link []byte, title []byte, alt []byte) {
	// Not rendered
	r.coallesce = false
}

// LineBreak dummy function to proceed with rendering
func (r *MarkdownStripper) LineBreak(out *bytes.Buffer) {
	// Not rendered
	r.coallesce = false
}

// Link dummy function to proceed with rendering
func (r *MarkdownStripper) Link(out *bytes.Buffer, link []byte, title []byte, content []byte) {
	r.processLink(out, link, content)
}

// RawHtmlTag dummy function to proceed with rendering
func (r *MarkdownStripper) RawHtmlTag(out *bytes.Buffer, tag []byte) { //nolint
	// Not rendered
	r.coallesce = false
}

// TripleEmphasis dummy function to proceed with rendering
func (r *MarkdownStripper) TripleEmphasis(out *bytes.Buffer, text []byte) {
	r.processString(out, text, false)
}

// StrikeThrough dummy function to proceed with rendering
func (r *MarkdownStripper) StrikeThrough(out *bytes.Buffer, text []byte) {
	r.processString(out, text, false)
}

// FootnoteRef dummy function to proceed with rendering
func (r *MarkdownStripper) FootnoteRef(out *bytes.Buffer, ref []byte, id int) {
	// Not rendered
	r.coallesce = false
}

// Low-level callbacks

// Entity dummy function to proceed with rendering
func (r *MarkdownStripper) Entity(out *bytes.Buffer, entity []byte) {
	// FIXME: literal entities are not parsed; perhaps they should
	r.coallesce = false
}

// NormalText dummy function to proceed with rendering
func (r *MarkdownStripper) NormalText(out *bytes.Buffer, text []byte) {
	r.processString(out, text, true)
}

// Header and footer

// DocumentHeader dummy function to proceed with rendering
func (r *MarkdownStripper) DocumentHeader(out *bytes.Buffer) {
	r.coallesce = false
}

// DocumentFooter dummy function to proceed with rendering
func (r *MarkdownStripper) DocumentFooter(out *bytes.Buffer) {
	r.coallesce = false
}

// GetFlags returns rendering flags
func (r *MarkdownStripper) GetFlags() int {
	return 0
}

//revive:enable:var-naming

func doubleSpace(out *bytes.Buffer) {
	if out.Len() > 0 {
		out.WriteByte('\n')
	}
}

func (r *MarkdownStripper) processString(out *bytes.Buffer, text []byte, coallesce bool) {
	// Always break-up words
	if !coallesce || !r.coallesce {
		doubleSpace(out)
	}
	out.Write(text)
	r.coallesce = coallesce
}
func (r *MarkdownStripper) processLink(out *bytes.Buffer, link []byte, content []byte) {
	// Links are processed out of band
	r.links = append(r.links, string(link))
	r.coallesce = false
}

// GetLinks returns the list of link data collected while parsing
func (r *MarkdownStripper) GetLinks() []string {
	return r.links
}
