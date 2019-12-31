package extension

import (
	"bytes"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"strconv"
)

var footnoteListKey = parser.NewContextKey()

type footnoteBlockParser struct {
}

var defaultFootnoteBlockParser = &footnoteBlockParser{}

// NewFootnoteBlockParser returns a new parser.BlockParser that can parse
// footnotes of the Markdown(PHP Markdown Extra) text.
func NewFootnoteBlockParser() parser.BlockParser {
	return defaultFootnoteBlockParser
}

func (b *footnoteBlockParser) Trigger() []byte {
	return []byte{'['}
}

func (b *footnoteBlockParser) Open(parent gast.Node, reader text.Reader, pc parser.Context) (gast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || line[pos] != '[' {
		return nil, parser.NoChildren
	}
	pos++
	if pos > len(line)-1 || line[pos] != '^' {
		return nil, parser.NoChildren
	}
	open := pos + 1
	closes := 0
	closure := util.FindClosure(line[pos+1:], '[', ']', false, false)
	closes = pos + 1 + closure
	next := closes + 1
	if closure > -1 {
		if next >= len(line) || line[next] != ':' {
			return nil, parser.NoChildren
		}
	} else {
		return nil, parser.NoChildren
	}
	padding := segment.Padding
	label := reader.Value(text.NewSegment(segment.Start+open-padding, segment.Start+closes-padding))
	if util.IsBlank(label) {
		return nil, parser.NoChildren
	}
	item := ast.NewFootnote(label)

	pos = next + 1 - padding
	if pos >= len(line) {
		reader.Advance(pos)
		return item, parser.NoChildren
	}
	reader.AdvanceAndSetPadding(pos, padding)
	return item, parser.HasChildren
}

func (b *footnoteBlockParser) Continue(node gast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, _ := reader.PeekLine()
	if util.IsBlank(line) {
		return parser.Continue | parser.HasChildren
	}
	childpos, padding := util.IndentPosition(line, reader.LineOffset(), 4)
	if childpos < 0 {
		return parser.Close
	}
	reader.AdvanceAndSetPadding(childpos, padding)
	return parser.Continue | parser.HasChildren
}

func (b *footnoteBlockParser) Close(node gast.Node, reader text.Reader, pc parser.Context) {
	var list *ast.FootnoteList
	if tlist := pc.Get(footnoteListKey); tlist != nil {
		list = tlist.(*ast.FootnoteList)
	} else {
		list = ast.NewFootnoteList()
		pc.Set(footnoteListKey, list)
		node.Parent().InsertBefore(node.Parent(), node, list)
	}
	node.Parent().RemoveChild(node.Parent(), node)
	list.AppendChild(list, node)
}

func (b *footnoteBlockParser) CanInterruptParagraph() bool {
	return true
}

func (b *footnoteBlockParser) CanAcceptIndentedLine() bool {
	return false
}

type footnoteParser struct {
}

var defaultFootnoteParser = &footnoteParser{}

// NewFootnoteParser returns a new parser.InlineParser that can parse
// footnote links of the Markdown(PHP Markdown Extra) text.
func NewFootnoteParser() parser.InlineParser {
	return defaultFootnoteParser
}

func (s *footnoteParser) Trigger() []byte {
	// footnote syntax probably conflict with the image syntax.
	// So we need trigger this parser with '!'.
	return []byte{'!', '['}
}

func (s *footnoteParser) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, segment := block.PeekLine()
	pos := 1
	if len(line) > 0 && line[0] == '!' {
		pos++
	}
	if pos >= len(line) || line[pos] != '^' {
		return nil
	}
	pos++
	if pos >= len(line) {
		return nil
	}
	open := pos
	closure := util.FindClosure(line[pos:], '[', ']', false, false)
	if closure < 0 {
		return nil
	}
	closes := pos + closure
	value := block.Value(text.NewSegment(segment.Start+open, segment.Start+closes))
	block.Advance(closes + 1)

	var list *ast.FootnoteList
	if tlist := pc.Get(footnoteListKey); tlist != nil {
		list = tlist.(*ast.FootnoteList)
	}
	if list == nil {
		return nil
	}
	index := 0
	for def := list.FirstChild(); def != nil; def = def.NextSibling() {
		d := def.(*ast.Footnote)
		if bytes.Equal(d.Ref, value) {
			if d.Index < 0 {
				list.Count += 1
				d.Index = list.Count
			}
			index = d.Index
			break
		}
	}
	if index == 0 {
		return nil
	}

	return ast.NewFootnoteLink(index)
}

type footnoteASTTransformer struct {
}

var defaultFootnoteASTTransformer = &footnoteASTTransformer{}

// NewFootnoteASTTransformer returns a new parser.ASTTransformer that
// insert a footnote list to the last of the document.
func NewFootnoteASTTransformer() parser.ASTTransformer {
	return defaultFootnoteASTTransformer
}

func (a *footnoteASTTransformer) Transform(node *gast.Document, reader text.Reader, pc parser.Context) {
	var list *ast.FootnoteList
	if tlist := pc.Get(footnoteListKey); tlist != nil {
		list = tlist.(*ast.FootnoteList)
	} else {
		return
	}
	pc.Set(footnoteListKey, nil)
	for footnote := list.FirstChild(); footnote != nil; {
		var container gast.Node = footnote
		next := footnote.NextSibling()
		if fc := container.LastChild(); fc != nil && gast.IsParagraph(fc) {
			container = fc
		}
		index := footnote.(*ast.Footnote).Index
		if index < 0 {
			list.RemoveChild(list, footnote)
		} else {
			container.AppendChild(container, ast.NewFootnoteBackLink(index))
		}
		footnote = next
	}
	list.SortChildren(func(n1, n2 gast.Node) int {
		if n1.(*ast.Footnote).Index < n2.(*ast.Footnote).Index {
			return -1
		}
		return 1
	})
	if list.Count <= 0 {
		list.Parent().RemoveChild(list.Parent(), list)
		return
	}

	node.AppendChild(node, list)
}

// FootnoteHTMLRenderer is a renderer.NodeRenderer implementation that
// renders FootnoteLink nodes.
type FootnoteHTMLRenderer struct {
	html.Config
}

// NewFootnoteHTMLRenderer returns a new FootnoteHTMLRenderer.
func NewFootnoteHTMLRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &FootnoteHTMLRenderer{
		Config: html.NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *FootnoteHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFootnoteLink, r.renderFootnoteLink)
	reg.Register(ast.KindFootnoteBackLink, r.renderFootnoteBackLink)
	reg.Register(ast.KindFootnote, r.renderFootnote)
	reg.Register(ast.KindFootnoteList, r.renderFootnoteList)
}

func (r *FootnoteHTMLRenderer) renderFootnoteLink(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		n := node.(*ast.FootnoteLink)
		is := strconv.Itoa(n.Index)
		_, _ = w.WriteString(`<sup id="fnref:`)
		_, _ = w.WriteString(is)
		_, _ = w.WriteString(`"><a href="#fn:`)
		_, _ = w.WriteString(is)
		_, _ = w.WriteString(`" class="footnote-ref" role="doc-noteref">`)
		_, _ = w.WriteString(is)
		_, _ = w.WriteString(`</a></sup>`)
	}
	return gast.WalkContinue, nil
}

func (r *FootnoteHTMLRenderer) renderFootnoteBackLink(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		n := node.(*ast.FootnoteBackLink)
		is := strconv.Itoa(n.Index)
		_, _ = w.WriteString(` <a href="#fnref:`)
		_, _ = w.WriteString(is)
		_, _ = w.WriteString(`" class="footnote-backref" role="doc-backlink">`)
		_, _ = w.WriteString("&#x21a9;&#xfe0e;")
		_, _ = w.WriteString(`</a>`)
	}
	return gast.WalkContinue, nil
}

func (r *FootnoteHTMLRenderer) renderFootnote(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	n := node.(*ast.Footnote)
	is := strconv.Itoa(n.Index)
	if entering {
		_, _ = w.WriteString(`<li id="fn:`)
		_, _ = w.WriteString(is)
		_, _ = w.WriteString(`" role="doc-endnote"`)
		if node.Attributes() != nil {
			html.RenderAttributes(w, node, html.ListItemAttributeFilter)
		}
		_, _ = w.WriteString(">\n")
	} else {
		_, _ = w.WriteString("</li>\n")
	}
	return gast.WalkContinue, nil
}

func (r *FootnoteHTMLRenderer) renderFootnoteList(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	tag := "section"
	if r.Config.XHTML {
		tag = "div"
	}
	if entering {
		_, _ = w.WriteString("<")
		_, _ = w.WriteString(tag)
		_, _ = w.WriteString(` class="footnotes" role="doc-endnotes"`)
		if node.Attributes() != nil {
			html.RenderAttributes(w, node, html.GlobalAttributeFilter)
		}
		_ = w.WriteByte('>')
		if r.Config.XHTML {
			_, _ = w.WriteString("\n<hr />\n")
		} else {
			_, _ = w.WriteString("\n<hr>\n")
		}
		_, _ = w.WriteString("<ol>\n")
	} else {
		_, _ = w.WriteString("</ol>\n")
		_, _ = w.WriteString("</")
		_, _ = w.WriteString(tag)
		_, _ = w.WriteString(">\n")
	}
	return gast.WalkContinue, nil
}

type footnote struct {
}

// Footnote is an extension that allow you to use PHP Markdown Extra Footnotes.
var Footnote = &footnote{}

func (e *footnote) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(
			util.Prioritized(NewFootnoteBlockParser(), 999),
		),
		parser.WithInlineParsers(
			util.Prioritized(NewFootnoteParser(), 101),
		),
		parser.WithASTTransformers(
			util.Prioritized(NewFootnoteASTTransformer(), 999),
		),
	)
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewFootnoteHTMLRenderer(), 500),
	))
}
