// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/internal"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ASTTransformer is a default transformer of the goldmark tree.
type ASTTransformer struct {
	renderInternal *internal.RenderInternal
	attentionTypes container.Set[string]
}

func NewASTTransformer(renderInternal *internal.RenderInternal) *ASTTransformer {
	return &ASTTransformer{
		renderInternal: renderInternal,
		attentionTypes: container.SetOf("note", "tip", "important", "warning", "caution"),
	}
}

func (g *ASTTransformer) applyElementDir(n ast.Node) {
	if !markup.RenderBehaviorForTesting.DisableAdditionalAttributes {
		n.SetAttributeString("dir", "auto")
	}
}

// Transform transforms the given AST tree.
func (g *ASTTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	firstChild := node.FirstChild()
	tocMode := ""
	ctx := pc.Get(renderContextKey).(*markup.RenderContext)
	rc := pc.Get(renderConfigKey).(*RenderConfig)

	tocList := make([]Header, 0, 20)
	if rc.yamlNode != nil {
		metaNode := rc.toMetaNode(g)
		if metaNode != nil {
			node.InsertBefore(node, firstChild, metaNode)
		}
		tocMode = rc.TOC
	}

	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch v := n.(type) {
		case *ast.Heading:
			g.transformHeading(ctx, v, reader, &tocList)
		case *ast.Paragraph:
			g.applyElementDir(v)
		case *ast.List:
			g.transformList(ctx, v, rc)
		case *ast.Text:
			if v.SoftLineBreak() && !v.HardLineBreak() {
				newLineHardBreak := ctx.RenderOptions.Metas["markdownNewLineHardBreak"] == "true"
				v.SetHardLineBreak(newLineHardBreak)
			}
		case *ast.CodeSpan:
			g.transformCodeSpan(ctx, v, reader)
		case *ast.Blockquote:
			return g.transformBlockquote(v, reader)
		}
		return ast.WalkContinue, nil
	})

	showTocInMain := tocMode == "true" /* old behavior, in main view */ || tocMode == "main"
	showTocInSidebar := !showTocInMain && tocMode != "false" // not hidden, not main, then show it in sidebar
	if len(tocList) > 0 && (showTocInMain || showTocInSidebar) {
		if showTocInMain {
			tocNode := createTOCNode(tocList, rc.Lang, nil)
			node.InsertBefore(node, firstChild, tocNode)
		} else {
			tocNode := createTOCNode(tocList, rc.Lang, map[string]string{"open": "open"})
			ctx.SidebarTocNode = tocNode
		}
	}

	if len(rc.Lang) > 0 {
		node.SetAttributeString("lang", []byte(rc.Lang))
	}
}

// NewHTMLRenderer creates a HTMLRenderer to render in the gitea form.
func NewHTMLRenderer(renderInternal *internal.RenderInternal, opts ...html.Option) renderer.NodeRenderer {
	r := &HTMLRenderer{
		renderInternal: renderInternal,
		Config:         html.NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// HTMLRenderer is a renderer.NodeRenderer implementation that
// renders gitea specific features.
type HTMLRenderer struct {
	html.Config
	renderInternal *internal.RenderInternal
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindDocument, r.renderDocument)
	reg.Register(KindDetails, r.renderDetails)
	reg.Register(KindSummary, r.renderSummary)
	reg.Register(ast.KindCodeSpan, r.renderCodeSpan)
	reg.Register(KindAttention, r.renderAttention)
	reg.Register(KindTaskCheckBoxListItem, r.renderTaskCheckBoxListItem)
	reg.Register(east.KindTaskCheckBox, r.renderTaskCheckBox)
	reg.Register(KindRawHTML, r.renderRawHTML)
}

func (r *HTMLRenderer) renderDocument(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Document)

	if val, has := n.AttributeString("lang"); has {
		var err error
		if entering {
			_, err = w.WriteString("<div")
			if err == nil {
				_, err = fmt.Fprintf(w, ` lang=%q`, val)
			}
			if err == nil {
				_, err = w.WriteRune('>')
			}
		} else {
			_, err = w.WriteString("</div>")
		}

		if err != nil {
			return ast.WalkStop, err
		}
	}

	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderDetails(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	var err error
	if entering {
		if _, err = w.WriteString("<details"); err != nil {
			return ast.WalkStop, err
		}
		html.RenderAttributes(w, node, nil)
		_, err = w.WriteString(">")
	} else {
		_, err = w.WriteString("</details>")
	}

	if err != nil {
		return ast.WalkStop, err
	}

	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderSummary(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	var err error
	if entering {
		_, err = w.WriteString("<summary>")
	} else {
		_, err = w.WriteString("</summary>")
	}

	if err != nil {
		return ast.WalkStop, err
	}

	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*RawHTML)
	_, err := w.WriteString(string(r.renderInternal.ProtectSafeAttrs(n.rawHTML)))
	if err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkContinue, nil
}
