// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/russross/blackfriday/v2"
)

// Renderer is a extended version of underlying render object.
type Renderer struct {
	blackfriday.Renderer
	URLPrefix string
	IsWiki    bool
}

var byteMailto = []byte("mailto:")

var htmlEscaper = [256][]byte{
	'&': []byte("&amp;"),
	'<': []byte("&lt;"),
	'>': []byte("&gt;"),
	'"': []byte("&quot;"),
}

func escapeHTML(w io.Writer, s []byte) {
	var start, end int
	for end < len(s) {
		escSeq := htmlEscaper[s[end]]
		if escSeq != nil {
			_, _ = w.Write(s[start:end])
			_, _ = w.Write(escSeq)
			start = end + 1
		}
		end++
	}
	if start < len(s) && end <= len(s) {
		_, _ = w.Write(s[start:end])
	}
}

// RenderNode is a default renderer of a single node of a syntax tree. For
// block nodes it will be called twice: first time with entering=true, second
// time with entering=false, so that it could know when it's working on an open
// tag and when on close. It writes the result to w.
//
// The return value is a way to tell the calling walker to adjust its walk
// pattern: e.g. it can terminate the traversal by returning Terminate. Or it
// can ask the walker to skip a subtree of this node by returning SkipChildren.
// The typical behavior is to return GoToNext, which asks for the usual
// traversal to the next node.
func (r *Renderer) RenderNode(w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	switch node.Type {
	case blackfriday.Image:
		prefix := r.URLPrefix
		if r.IsWiki {
			prefix = util.URLJoin(prefix, "wiki", "raw")
		}
		prefix = strings.Replace(prefix, "/src/", "/media/", 1)
		link := node.LinkData.Destination
		if len(link) > 0 && !markup.IsLink(link) {
			lnk := string(link)
			lnk = util.URLJoin(prefix, lnk)
			lnk = strings.Replace(lnk, " ", "+", -1)
			link = []byte(lnk)
		}
		node.LinkData.Destination = link
		// Render link around image only if parent is not link already
		if node.Parent != nil && node.Parent.Type != blackfriday.Link {
			if entering {
				_, _ = w.Write([]byte(`<a href="`))
				escapeHTML(w, link)
				_, _ = w.Write([]byte(`">`))
				return r.Renderer.RenderNode(w, node, entering)
			}
			s := r.Renderer.RenderNode(w, node, entering)
			_, _ = w.Write([]byte(`</a>`))
			return s
		}
		return r.Renderer.RenderNode(w, node, entering)
	case blackfriday.Link:
		// special case: this is not a link, a hash link or a mailto:, so it's a
		// relative URL
		link := node.LinkData.Destination
		if len(link) > 0 && !markup.IsLink(link) &&
			link[0] != '#' && !bytes.HasPrefix(link, byteMailto) &&
			node.LinkData.Footnote == nil {
			lnk := string(link)
			if r.IsWiki {
				lnk = util.URLJoin("wiki", lnk)
			}
			link = []byte(util.URLJoin(r.URLPrefix, lnk))
		}
		node.LinkData.Destination = link
		return r.Renderer.RenderNode(w, node, entering)
	case blackfriday.Text:
		isListItem := false
		for n := node.Parent; n != nil; n = n.Parent {
			if n.Type == blackfriday.Item {
				isListItem = true
				break
			}
		}
		if isListItem {
			text := node.Literal
			switch {
			case bytes.HasPrefix(text, []byte("[ ] ")):
				_, _ = w.Write([]byte(`<span class="ui fitted disabled checkbox"><input type="checkbox" disabled="disabled" /><label /></span>`))
				text = text[3:]
			case bytes.HasPrefix(text, []byte("[x] ")):
				_, _ = w.Write([]byte(`<span class="ui checked fitted disabled checkbox"><input type="checkbox" checked="" disabled="disabled" /><label /></span>`))
				text = text[3:]
			}
			node.Literal = text
		}
	}
	return r.Renderer.RenderNode(w, node, entering)
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
		blackfriday.AutoHeadingIDs
	blackfridayHTMLFlags = 0 |
		blackfriday.Smartypants
)

// RenderRaw renders Markdown to HTML without handling special links.
func RenderRaw(body []byte, urlPrefix string, wikiMarkdown bool) []byte {
	renderer := &Renderer{
		Renderer: blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
			Flags: blackfridayHTMLFlags,
		}),
		URLPrefix: urlPrefix,
		IsWiki:    wikiMarkdown,
	}

	exts := blackfridayExtensions
	if setting.Markdown.EnableHardLineBreak {
		exts |= blackfriday.HardLineBreak
	}

	// Need to normalize EOL to UNIX LF to have consistent results in rendering
	body = blackfriday.Run(util.NormalizeEOL(body), blackfriday.WithRenderer(renderer), blackfriday.WithExtensions(exts))
	return markup.SanitizeBytes(body)
}

var (
	// MarkupName describes markup's name
	MarkupName = "markdown"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return MarkupName
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return setting.Markdown.FileExtensions
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return RenderRaw(rawBytes, urlPrefix, isWiki)
}

// Render renders Markdown to HTML with all specific handling stuff.
func Render(rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return markup.Render("a.md", rawBytes, urlPrefix, metas)
}

// RenderString renders Markdown to HTML with special links and returns string type.
func RenderString(raw, urlPrefix string, metas map[string]string) string {
	return markup.RenderString("a.md", raw, urlPrefix, metas)
}

// RenderWiki renders markdown wiki page to HTML and return HTML string
func RenderWiki(rawBytes []byte, urlPrefix string, metas map[string]string) string {
	return markup.RenderWiki("a.md", rawBytes, urlPrefix, metas)
}

// IsMarkdownFile reports whether name looks like a Markdown file
// based on its extension.
func IsMarkdownFile(name string) bool {
	return markup.IsMarkupFile(name, MarkupName)
}
