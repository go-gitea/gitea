// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
	giteautil "code.gitea.io/gitea/modules/util"

	"github.com/microcosm-cc/bluemonday/css"
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
	AttentionTypes container.Set[string]
}

func NewASTTransformer() *ASTTransformer {
	return &ASTTransformer{
		AttentionTypes: container.SetOf("note", "tip", "important", "warning", "caution"),
	}
}

func (g *ASTTransformer) applyElementDir(n ast.Node) {
	if markup.DefaultProcessorHelper.ElementDir != "" {
		n.SetAttributeString("dir", []byte(markup.DefaultProcessorHelper.ElementDir))
	}
}

// Transform transforms the given AST tree.
func (g *ASTTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	firstChild := node.FirstChild()
	tocMode := ""
	ctx := pc.Get(renderContextKey).(*markup.RenderContext)
	rc := pc.Get(renderConfigKey).(*RenderConfig)

	tocList := make([]markup.Header, 0, 20)
	if rc.yamlNode != nil {
		metaNode := rc.toMetaNode()
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
			for _, attr := range v.Attributes() {
				if _, ok := attr.Value.([]byte); !ok {
					v.SetAttribute(attr.Name, []byte(fmt.Sprintf("%v", attr.Value)))
				}
			}
			txt := n.Text(reader.Source())
			header := markup.Header{
				Text:  util.BytesToReadOnlyString(txt),
				Level: v.Level,
			}
			if id, found := v.AttributeString("id"); found {
				header.ID = util.BytesToReadOnlyString(id.([]byte))
			}
			tocList = append(tocList, header)
			g.applyElementDir(v)
		case *ast.Paragraph:
			g.applyElementDir(v)
		case *ast.Image:
			// Images need two things:
			//
			// 1. Their src needs to munged to be a real value
			// 2. If they're not wrapped with a link they need a link wrapper

			// Check if the destination is a real link
			if len(v.Destination) > 0 && !markup.IsFullURLBytes(v.Destination) {
				v.Destination = []byte(giteautil.URLJoin(
					ctx.Links.ResolveMediaLink(ctx.IsWiki),
					strings.TrimLeft(string(v.Destination), "/"),
				))
			}

			parent := n.Parent()
			// Create a link around image only if parent is not already a link
			if _, ok := parent.(*ast.Link); !ok && parent != nil {
				next := n.NextSibling()

				// Create a link wrapper
				wrap := ast.NewLink()
				wrap.Destination = v.Destination
				wrap.Title = v.Title
				wrap.SetAttributeString("target", []byte("_blank"))

				// Duplicate the current image node
				image := ast.NewImage(ast.NewLink())
				image.Destination = v.Destination
				image.Title = v.Title
				for _, attr := range v.Attributes() {
					image.SetAttribute(attr.Name, attr.Value)
				}
				for child := v.FirstChild(); child != nil; {
					next := child.NextSibling()
					image.AppendChild(image, child)
					child = next
				}

				// Append our duplicate image to the wrapper link
				wrap.AppendChild(wrap, image)

				// Wire in the next sibling
				wrap.SetNextSibling(next)

				// Replace the current node with the wrapper link
				parent.ReplaceChild(parent, n, wrap)

				// But most importantly ensure the next sibling is still on the old image too
				v.SetNextSibling(next)
			}
		case *ast.Link:
			// Links need their href to munged to be a real value
			link := v.Destination
			isAnchorFragment := len(link) > 0 && link[0] == '#'
			if !isAnchorFragment && !markup.IsFullURLBytes(link) {
				base := ctx.Links.Base
				if ctx.IsWiki {
					base = ctx.Links.WikiLink()
				} else if ctx.Links.HasBranchInfo() {
					base = ctx.Links.SrcLink()
				}
				link = []byte(giteautil.URLJoin(base, string(link)))
			}
			if isAnchorFragment {
				link = []byte("#user-content-" + string(link)[1:])
			}
			v.Destination = link
		case *ast.List:
			if v.HasChildren() {
				children := make([]ast.Node, 0, v.ChildCount())
				child := v.FirstChild()
				for child != nil {
					children = append(children, child)
					child = child.NextSibling()
				}
				v.RemoveChildren(v)

				for _, child := range children {
					listItem := child.(*ast.ListItem)
					if !child.HasChildren() || !child.FirstChild().HasChildren() {
						v.AppendChild(v, child)
						continue
					}
					taskCheckBox, ok := child.FirstChild().FirstChild().(*east.TaskCheckBox)
					if !ok {
						v.AppendChild(v, child)
						continue
					}
					newChild := NewTaskCheckBoxListItem(listItem)
					newChild.IsChecked = taskCheckBox.IsChecked
					newChild.SetAttributeString("class", []byte("task-list-item"))
					segments := newChild.FirstChild().Lines()
					if segments.Len() > 0 {
						segment := segments.At(0)
						newChild.SourcePosition = rc.metaLength + segment.Start
					}
					v.AppendChild(v, newChild)
				}
			}
			g.applyElementDir(v)
		case *ast.Text:
			if v.SoftLineBreak() && !v.HardLineBreak() {
				if ctx.Metas["mode"] != "document" {
					v.SetHardLineBreak(setting.Markdown.EnableHardLineBreakInComments)
				} else {
					v.SetHardLineBreak(setting.Markdown.EnableHardLineBreakInDocuments)
				}
			}
		case *ast.CodeSpan:
			colorContent := n.Text(reader.Source())
			if css.ColorHandler(strings.ToLower(string(colorContent))) {
				v.AppendChild(v, NewColorPreview(colorContent))
			}
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

type prefixedIDs struct {
	values container.Set[string]
}

// Generate generates a new element id.
func (p *prefixedIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	dft := []byte("id")
	if kind == ast.KindHeading {
		dft = []byte("heading")
	}
	return p.GenerateWithDefault(value, dft)
}

// GenerateWithDefault generates a new element id.
func (p *prefixedIDs) GenerateWithDefault(value, dft []byte) []byte {
	result := common.CleanValue(value)
	if len(result) == 0 {
		result = dft
	}
	if !bytes.HasPrefix(result, []byte("user-content-")) {
		result = append([]byte("user-content-"), result...)
	}
	if p.values.Add(util.BytesToReadOnlyString(result)) {
		return result
	}
	for i := 1; ; i++ {
		newResult := fmt.Sprintf("%s-%d", result, i)
		if p.values.Add(newResult) {
			return []byte(newResult)
		}
	}
}

// Put puts a given element id to the used ids table.
func (p *prefixedIDs) Put(value []byte) {
	p.values.Add(util.BytesToReadOnlyString(value))
}

func newPrefixedIDs() *prefixedIDs {
	return &prefixedIDs{
		values: make(container.Set[string]),
	}
}

// NewHTMLRenderer creates a HTMLRenderer to render
// in the gitea form.
func NewHTMLRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &HTMLRenderer{
		Config:      html.NewConfig(),
		reValidName: regexp.MustCompile("^[a-z ]+$"),
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
	reValidName *regexp.Regexp
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindDocument, r.renderDocument)
	reg.Register(KindDetails, r.renderDetails)
	reg.Register(KindSummary, r.renderSummary)
	reg.Register(KindIcon, r.renderIcon)
	reg.Register(ast.KindCodeSpan, r.renderCodeSpan)
	reg.Register(KindAttention, r.renderAttention)
	reg.Register(KindTaskCheckBoxListItem, r.renderTaskCheckBoxListItem)
	reg.Register(east.KindTaskCheckBox, r.renderTaskCheckBox)
}

// renderCodeSpan renders CodeSpan elements (like goldmark upstream does) but also renders ColorPreview elements.
// See #21474 for reference
func (r *HTMLRenderer) renderCodeSpan(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if n.Attributes() != nil {
			_, _ = w.WriteString("<code")
			html.RenderAttributes(w, n, html.CodeAttributeFilter)
			_ = w.WriteByte('>')
		} else {
			_, _ = w.WriteString("<code>")
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch v := c.(type) {
			case *ast.Text:
				segment := v.Segment
				value := segment.Value(source)
				if bytes.HasSuffix(value, []byte("\n")) {
					r.Writer.RawWrite(w, value[:len(value)-1])
					r.Writer.RawWrite(w, []byte(" "))
				} else {
					r.Writer.RawWrite(w, value)
				}
			case *ColorPreview:
				_, _ = w.WriteString(fmt.Sprintf(`<span class="color-preview" style="background-color: %v"></span>`, string(v.Color)))
			}
		}
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString("</code>")
	return ast.WalkContinue, nil
}

// renderAttention renders a quote marked with i.e. "> **Note**" or "> **Warning**" with a corresponding svg
func (r *HTMLRenderer) renderAttention(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*Attention)
		var octiconName string
		switch n.AttentionType {
		case "tip":
			octiconName = "light-bulb"
		case "important":
			octiconName = "report"
		case "warning":
			octiconName = "alert"
		case "caution":
			octiconName = "stop"
		default: // including "note"
			octiconName = "info"
		}
		_, _ = w.WriteString(string(svg.RenderHTML("octicon-"+octiconName, 16, "attention-icon attention-"+n.AttentionType)))
	}
	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderDocument(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Document)

	if val, has := n.AttributeString("lang"); has {
		var err error
		if entering {
			_, err = w.WriteString("<div")
			if err == nil {
				_, err = w.WriteString(fmt.Sprintf(` lang=%q`, val))
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

func (r *HTMLRenderer) renderIcon(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*Icon)

	name := strings.TrimSpace(strings.ToLower(string(n.Name)))

	if len(name) == 0 {
		// skip this
		return ast.WalkContinue, nil
	}

	if !r.reValidName.MatchString(name) {
		// skip this
		return ast.WalkContinue, nil
	}

	var err error
	_, err = w.WriteString(fmt.Sprintf(`<i class="icon %s"></i>`, name))
	if err != nil {
		return ast.WalkStop, err
	}

	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderTaskCheckBoxListItem(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*TaskCheckBoxListItem)
	if entering {
		if n.Attributes() != nil {
			_, _ = w.WriteString("<li")
			html.RenderAttributes(w, n, html.ListItemAttributeFilter)
			_ = w.WriteByte('>')
		} else {
			_, _ = w.WriteString("<li>")
		}
		fmt.Fprintf(w, `<input type="checkbox" disabled="" data-source-position="%d"`, n.SourcePosition)
		if n.IsChecked {
			_, _ = w.WriteString(` checked=""`)
		}
		if r.XHTML {
			_, _ = w.WriteString(` />`)
		} else {
			_ = w.WriteByte('>')
		}
		fc := n.FirstChild()
		if fc != nil {
			if _, ok := fc.(*ast.TextBlock); !ok {
				_ = w.WriteByte('\n')
			}
		}
	} else {
		_, _ = w.WriteString("</li>\n")
	}
	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}
