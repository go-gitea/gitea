// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/common"
	giteautil "code.gitea.io/gitea/modules/util"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var byteMailto = []byte("mailto:")

// GiteaASTTransformer is a default transformer of the goldmark tree.
type GiteaASTTransformer struct{}

// Transform transforms the given AST tree.
func (g *GiteaASTTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch v := n.(type) {
		case *ast.Image:
			// Images need two things:
			//
			// 1. Their src needs to munged to be a real value
			// 2. If they're not wrapped with a link they need a link wrapper

			// Check if the destination is a real link
			link := v.Destination
			if len(link) > 0 && !markup.IsLink(link) {
				prefix := pc.Get(urlPrefixKey).(string)
				if pc.Get(isWikiKey).(bool) {
					prefix = giteautil.URLJoin(prefix, "wiki", "raw")
				}
				prefix = strings.Replace(prefix, "/src/", "/media/", 1)

				lnk := string(link)
				lnk = giteautil.URLJoin(prefix, lnk)
				link = []byte(lnk)
			}
			v.Destination = link

			parent := n.Parent()
			// Create a link around image only if parent is not already a link
			if _, ok := parent.(*ast.Link); !ok && parent != nil {
				wrap := ast.NewLink()
				wrap.Destination = link
				wrap.Title = v.Title
				parent.ReplaceChild(parent, n, wrap)
				wrap.AppendChild(wrap, n)
			}
		case *ast.Link:
			// Links need their href to munged to be a real value
			link := v.Destination
			if len(link) > 0 && !markup.IsLink(link) &&
				link[0] != '#' && !bytes.HasPrefix(link, byteMailto) {
				// special case: this is not a link, a hash link or a mailto:, so it's a
				// relative URL
				lnk := string(link)
				if pc.Get(isWikiKey).(bool) {
					lnk = giteautil.URLJoin("wiki", lnk)
				}
				link = []byte(giteautil.URLJoin(pc.Get(urlPrefixKey).(string), lnk))
			}
			if len(link) > 0 && link[0] == '#' {
				link = []byte("#user-content-" + string(link)[1:])
			}
			v.Destination = link
		}
		return ast.WalkContinue, nil
	})
}

type prefixedIDs struct {
	values map[string]bool
}

// Generate generates a new element id.
func (p *prefixedIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	dft := []byte("id")
	if kind == ast.KindHeading {
		dft = []byte("heading")
	}
	return p.GenerateWithDefault(value, dft)
}

// Generate generates a new element id.
func (p *prefixedIDs) GenerateWithDefault(value []byte, dft []byte) []byte {
	result := common.CleanValue(value)
	if len(result) == 0 {
		result = dft
	}
	if !bytes.HasPrefix(result, []byte("user-content-")) {
		result = append([]byte("user-content-"), result...)
	}
	if _, ok := p.values[util.BytesToReadOnlyString(result)]; !ok {
		p.values[util.BytesToReadOnlyString(result)] = true
		return result
	}
	for i := 1; ; i++ {
		newResult := fmt.Sprintf("%s-%d", result, i)
		if _, ok := p.values[newResult]; !ok {
			p.values[newResult] = true
			return []byte(newResult)
		}
	}
}

// Put puts a given element id to the used ids table.
func (p *prefixedIDs) Put(value []byte) {
	p.values[util.BytesToReadOnlyString(value)] = true
}

func newPrefixedIDs() *prefixedIDs {
	return &prefixedIDs{
		values: map[string]bool{},
	}
}

// NewTaskCheckBoxHTMLRenderer creates a TaskCheckBoxHTMLRenderer to render tasklists
// in the gitea form.
func NewTaskCheckBoxHTMLRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &TaskCheckBoxHTMLRenderer{
		Config: html.NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// TaskCheckBoxHTMLRenderer is a renderer.NodeRenderer implementation that
// renders checkboxes in list items.
// Overrides the default goldmark one to present the gitea format
type TaskCheckBoxHTMLRenderer struct {
	html.Config
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *TaskCheckBoxHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(east.KindTaskCheckBox, r.renderTaskCheckBox)
}

func (r *TaskCheckBoxHTMLRenderer) renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*east.TaskCheckBox)

	end := ">"
	if r.XHTML {
		end = " />"
	}
	var err error
	if n.IsChecked {
		_, err = w.WriteString(`<span class="ui fitted disabled checkbox"><input type="checkbox" disabled="disabled"` + end + `<label` + end + `</span>`)
	} else {
		_, err = w.WriteString(`<span class="ui checked fitted disabled checkbox"><input type="checkbox" checked="" disabled="disabled"` + end + `<label` + end + `</span>`)
	}
	if err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkContinue, nil
}
