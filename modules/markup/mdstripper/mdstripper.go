// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mdstripper

import (
	"bytes"
	"io"

	"github.com/russross/blackfriday/v2"
)

// MarkdownStripper extends blackfriday.Renderer
type MarkdownStripper struct {
	links     []string
	coallesce bool
	empty     bool
}

const (
	blackfridayExtensions = 0 |
		blackfriday.NoIntraEmphasis |
		blackfriday.Tables |
		blackfriday.FencedCode |
		blackfriday.Strikethrough |
		blackfriday.NoEmptyLineBeforeBlock |
		blackfriday.DefinitionLists |
		blackfriday.Footnotes |
		blackfriday.HeadingIDs |
		blackfriday.AutoHeadingIDs |
		// Not included in modules/markup/markdown/markdown.go;
		// required here to process inline links
		blackfriday.Autolink
)

// StripMarkdown parses markdown content by removing all markup and code blocks
//	in order to extract links and other references
func StripMarkdown(rawBytes []byte) (string, []string) {
	buf, links := StripMarkdownBytes(rawBytes)
	return string(buf), links
}

// StripMarkdownBytes parses markdown content by removing all markup and code blocks
//	in order to extract links and other references
func StripMarkdownBytes(rawBytes []byte) ([]byte, []string) {
	stripper := &MarkdownStripper{
		links: make([]string, 0, 10),
		empty: true,
	}

	parser := blackfriday.New(blackfriday.WithRenderer(stripper), blackfriday.WithExtensions(blackfridayExtensions))
	ast := parser.Parse(rawBytes)
	var buf bytes.Buffer
	stripper.RenderHeader(&buf, ast)
	ast.Walk(func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		return stripper.RenderNode(&buf, node, entering)
	})
	stripper.RenderFooter(&buf, ast)
	return buf.Bytes(), stripper.GetLinks()
}

// RenderNode is the main rendering method. It will be called once for
// every leaf node and twice for every non-leaf node (first with
// entering=true, then with entering=false). The method should write its
// rendition of the node to the supplied writer w.
func (r *MarkdownStripper) RenderNode(w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	if !entering {
		return blackfriday.GoToNext
	}
	switch node.Type {
	case blackfriday.Text:
		r.processString(w, node.Literal, node.Parent == nil)
		return blackfriday.GoToNext
	case blackfriday.Link:
		r.processLink(w, node.LinkData.Destination)
		r.coallesce = false
		return blackfriday.SkipChildren
	}
	r.coallesce = false
	return blackfriday.GoToNext
}

// RenderHeader is a method that allows the renderer to produce some
// content preceding the main body of the output document.
func (r *MarkdownStripper) RenderHeader(w io.Writer, ast *blackfriday.Node) {
}

// RenderFooter is a symmetric counterpart of RenderHeader.
func (r *MarkdownStripper) RenderFooter(w io.Writer, ast *blackfriday.Node) {
}

func (r *MarkdownStripper) doubleSpace(w io.Writer) {
	if !r.empty {
		_, _ = w.Write([]byte{'\n'})
	}
}

func (r *MarkdownStripper) processString(w io.Writer, text []byte, coallesce bool) {
	// Always break-up words
	if !coallesce || !r.coallesce {
		r.doubleSpace(w)
	}
	_, _ = w.Write(text)
	r.coallesce = coallesce
	r.empty = false
}

func (r *MarkdownStripper) processLink(w io.Writer, link []byte) {
	// Links are processed out of band
	r.links = append(r.links, string(link))
	r.coallesce = false
}

// GetLinks returns the list of link data collected while parsing
func (r *MarkdownStripper) GetLinks() []string {
	return r.links
}
