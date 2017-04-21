// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"strings"

	"code.gitea.io/gitea/modules/markup"

	"github.com/russross/blackfriday"
)

// Renderer is a extended version of underlying render object.
type Renderer struct {
	blackfriday.Renderer
	URLPrefix string
	IsWiki    bool
}

// Link defines how formal links should be processed to produce corresponding HTML elements.
func (r *Renderer) Link(out *bytes.Buffer, link []byte, title []byte, content []byte) {
	if len(link) > 0 && !markup.IsLink(link) {
		if link[0] != '#' {
			lnk := string(link)
			if r.IsWiki {
				lnk = markup.URLJoin("wiki", lnk)
			}
			mLink := markup.URLJoin(r.URLPrefix, lnk)
			link = []byte(mLink)
		}
	}

	r.Renderer.Link(out, link, title, content)
}

// List renders markdown bullet or digit lists to HTML
func (r *Renderer) List(out *bytes.Buffer, text func() bool, flags int) {
	marker := out.Len()
	if out.Len() > 0 {
		out.WriteByte('\n')
	}

	if flags&blackfriday.LIST_TYPE_DEFINITION != 0 {
		out.WriteString("<dl>")
	} else if flags&blackfriday.LIST_TYPE_ORDERED != 0 {
		out.WriteString("<ol class='ui list'>")
	} else {
		out.WriteString("<ul class='ui list'>")
	}
	if !text() {
		out.Truncate(marker)
		return
	}
	if flags&blackfriday.LIST_TYPE_DEFINITION != 0 {
		out.WriteString("</dl>\n")
	} else if flags&blackfriday.LIST_TYPE_ORDERED != 0 {
		out.WriteString("</ol>\n")
	} else {
		out.WriteString("</ul>\n")
	}
}

// ListItem defines how list items should be processed to produce corresponding HTML elements.
func (r *Renderer) ListItem(out *bytes.Buffer, text []byte, flags int) {
	// Detect procedures to draw checkboxes.
	prefix := ""
	if bytes.HasPrefix(text, []byte("<p>")) {
		prefix = "<p>"
	}
	switch {
	case bytes.HasPrefix(text, []byte(prefix+"[ ] ")):
		text = append([]byte(`<div class="ui fitted disabled checkbox"><input type="checkbox" disabled="disabled" /><label /></div>`), text[3+len(prefix):]...)
	case bytes.HasPrefix(text, []byte(prefix+"[x] ")):
		text = append([]byte(`<div class="ui checked fitted disabled checkbox"><input type="checkbox" checked="" disabled="disabled" /><label /></div>`), text[3+len(prefix):]...)
	}
	if prefix != "" {
		text = bytes.Replace(text, []byte("</p>"), []byte{}, 1)
	}
	r.Renderer.ListItem(out, text, flags)
}

// Note: this section is for purpose of increase performance and
// reduce memory allocation at runtime since they are constant literals.
var (
	svgSuffix         = []byte(".svg")
	svgSuffixWithMark = []byte(".svg?")
)

// Image defines how images should be processed to produce corresponding HTML elements.
func (r *Renderer) Image(out *bytes.Buffer, link []byte, title []byte, alt []byte) {
	prefix := r.URLPrefix
	if r.IsWiki {
		prefix = markup.URLJoin(prefix, "wiki", "src")
	}
	prefix = strings.Replace(prefix, "/src/", "/raw/", 1)
	if len(link) > 0 {
		if markup.IsLink(link) {
			// External link with .svg suffix usually means CI status.
			// TODO: define a keyword to allow non-svg images render as external link.
			if bytes.HasSuffix(link, svgSuffix) || bytes.Contains(link, svgSuffixWithMark) {
				r.Renderer.Image(out, link, title, alt)
				return
			}
		} else {
			lnk := string(link)
			lnk = markup.URLJoin(prefix, lnk)
			lnk = strings.Replace(lnk, " ", "+", -1)
			link = []byte(lnk)
		}
	}

	out.WriteString(`<a href="`)
	out.Write(link)
	out.WriteString(`">`)
	r.Renderer.Image(out, link, title, alt)
	out.WriteString("</a>")
}
