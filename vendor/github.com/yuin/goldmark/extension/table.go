package extension

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var tableDelimRegexp = regexp.MustCompile(`^[\s\-\|\:]+$`)
var tableDelimLeft = regexp.MustCompile(`^\s*\:\-+\s*$`)
var tableDelimRight = regexp.MustCompile(`^\s*\-+\:\s*$`)
var tableDelimCenter = regexp.MustCompile(`^\s*\:\-+\:\s*$`)
var tableDelimNone = regexp.MustCompile(`^\s*\-+\s*$`)

type tableParagraphTransformer struct {
}

var defaultTableParagraphTransformer = &tableParagraphTransformer{}

// NewTableParagraphTransformer returns  a new ParagraphTransformer
// that can transform pargraphs into tables.
func NewTableParagraphTransformer() parser.ParagraphTransformer {
	return defaultTableParagraphTransformer
}

func (b *tableParagraphTransformer) Transform(node *gast.Paragraph, reader text.Reader, pc parser.Context) {
	lines := node.Lines()
	if lines.Len() < 2 {
		return
	}
	alignments := b.parseDelimiter(lines.At(1), reader)
	if alignments == nil {
		return
	}
	header := b.parseRow(lines.At(0), alignments, true, reader)
	if header == nil || len(alignments) != header.ChildCount() {
		return
	}
	table := ast.NewTable()
	table.Alignments = alignments
	table.AppendChild(table, ast.NewTableHeader(header))
	for i := 2; i < lines.Len(); i++ {
		table.AppendChild(table, b.parseRow(lines.At(i), alignments, false, reader))
	}
	node.Parent().InsertBefore(node.Parent(), node, table)
	node.Parent().RemoveChild(node.Parent(), node)
}

func (b *tableParagraphTransformer) parseRow(segment text.Segment, alignments []ast.Alignment, isHeader bool, reader text.Reader) *ast.TableRow {
	source := reader.Source()
	line := segment.Value(source)
	pos := 0
	pos += util.TrimLeftSpaceLength(line)
	limit := len(line)
	limit -= util.TrimRightSpaceLength(line)
	row := ast.NewTableRow(alignments)
	if len(line) > 0 && line[pos] == '|' {
		pos++
	}
	if len(line) > 0 && line[limit-1] == '|' {
		limit--
	}
	i := 0
	for ; pos < limit; i++ {
		alignment := ast.AlignNone
		if i >= len(alignments) {
			if !isHeader {
				return row
			}
		} else {
			alignment = alignments[i]
		}
		closure := util.FindClosure(line[pos:], byte(0), '|', true, false)
		if closure < 0 {
			closure = len(line[pos:])
		}
		node := ast.NewTableCell()
		seg := text.NewSegment(segment.Start+pos, segment.Start+pos+closure)
		seg = seg.TrimLeftSpace(source)
		seg = seg.TrimRightSpace(source)
		node.Lines().Append(seg)
		node.Alignment = alignment
		row.AppendChild(row, node)
		pos += closure + 1
	}
	for ; i < len(alignments); i++ {
		row.AppendChild(row, ast.NewTableCell())
	}
	return row
}

func (b *tableParagraphTransformer) parseDelimiter(segment text.Segment, reader text.Reader) []ast.Alignment {
	line := segment.Value(reader.Source())
	if !tableDelimRegexp.Match(line) {
		return nil
	}
	cols := bytes.Split(line, []byte{'|'})
	if util.IsBlank(cols[0]) {
		cols = cols[1:]
	}
	if len(cols) > 0 && util.IsBlank(cols[len(cols)-1]) {
		cols = cols[:len(cols)-1]
	}

	var alignments []ast.Alignment
	for _, col := range cols {
		if tableDelimLeft.Match(col) {
			alignments = append(alignments, ast.AlignLeft)
		} else if tableDelimRight.Match(col) {
			alignments = append(alignments, ast.AlignRight)
		} else if tableDelimCenter.Match(col) {
			alignments = append(alignments, ast.AlignCenter)
		} else if tableDelimNone.Match(col) {
			alignments = append(alignments, ast.AlignNone)
		} else {
			return nil
		}
	}
	return alignments
}

// TableHTMLRenderer is a renderer.NodeRenderer implementation that
// renders Table nodes.
type TableHTMLRenderer struct {
	html.Config
}

// NewTableHTMLRenderer returns a new TableHTMLRenderer.
func NewTableHTMLRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &TableHTMLRenderer{
		Config: html.NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *TableHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindTable, r.renderTable)
	reg.Register(ast.KindTableHeader, r.renderTableHeader)
	reg.Register(ast.KindTableRow, r.renderTableRow)
	reg.Register(ast.KindTableCell, r.renderTableCell)
}

// TableAttributeFilter defines attribute names which table elements can have.
var TableAttributeFilter = html.GlobalAttributeFilter.Extend(
	[]byte("align"),       // [Deprecated]
	[]byte("bgcolor"),     // [Deprecated]
	[]byte("border"),      // [Deprecated]
	[]byte("cellpadding"), // [Deprecated]
	[]byte("cellspacing"), // [Deprecated]
	[]byte("frame"),       // [Deprecated]
	[]byte("rules"),       // [Deprecated]
	[]byte("summary"),     // [Deprecated]
	[]byte("width"),       // [Deprecated]
)

func (r *TableHTMLRenderer) renderTable(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<table")
		if n.Attributes() != nil {
			html.RenderAttributes(w, n, TableAttributeFilter)
		}
		_, _ = w.WriteString(">\n")
	} else {
		_, _ = w.WriteString("</table>\n")
	}
	return gast.WalkContinue, nil
}

// TableHeaderAttributeFilter defines attribute names which <thead> elements can have.
var TableHeaderAttributeFilter = html.GlobalAttributeFilter.Extend(
	[]byte("align"),   // [Deprecated since HTML4] [Obsolete since HTML5]
	[]byte("bgcolor"), // [Not Standardized]
	[]byte("char"),    // [Deprecated since HTML4] [Obsolete since HTML5]
	[]byte("charoff"), // [Deprecated since HTML4] [Obsolete since HTML5]
	[]byte("valign"),  // [Deprecated since HTML4] [Obsolete since HTML5]
)

func (r *TableHTMLRenderer) renderTableHeader(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<thead")
		if n.Attributes() != nil {
			html.RenderAttributes(w, n, TableHeaderAttributeFilter)
		}
		_, _ = w.WriteString(">\n")
		_, _ = w.WriteString("<tr>\n") // Header <tr> has no separate handle
	} else {
		_, _ = w.WriteString("</tr>\n")
		_, _ = w.WriteString("</thead>\n")
		if n.NextSibling() != nil {
			_, _ = w.WriteString("<tbody>\n")
		}
	}
	return gast.WalkContinue, nil
}

// TableRowAttributeFilter defines attribute names which <tr> elements can have.
var TableRowAttributeFilter = html.GlobalAttributeFilter.Extend(
	[]byte("align"),   // [Obsolete since HTML5]
	[]byte("bgcolor"), // [Obsolete since HTML5]
	[]byte("char"),    // [Obsolete since HTML5]
	[]byte("charoff"), // [Obsolete since HTML5]
	[]byte("valign"),  // [Obsolete since HTML5]
)

func (r *TableHTMLRenderer) renderTableRow(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<tr")
		if n.Attributes() != nil {
			html.RenderAttributes(w, n, TableRowAttributeFilter)
		}
		_, _ = w.WriteString(">\n")
	} else {
		_, _ = w.WriteString("</tr>\n")
		if n.Parent().LastChild() == n {
			_, _ = w.WriteString("</tbody>\n")
		}
	}
	return gast.WalkContinue, nil
}

// TableThCellAttributeFilter defines attribute names which table <th> cells can have.
var TableThCellAttributeFilter = html.GlobalAttributeFilter.Extend(
	[]byte("abbr"), // [OK] Contains a short abbreviated description of the cell's content [NOT OK in <td>]

	[]byte("align"),   // [Obsolete since HTML5]
	[]byte("axis"),    // [Obsolete since HTML5]
	[]byte("bgcolor"), // [Not Standardized]
	[]byte("char"),    // [Obsolete since HTML5]
	[]byte("charoff"), // [Obsolete since HTML5]

	[]byte("colspan"), // [OK] Number of columns that the cell is to span
	[]byte("headers"), // [OK] This attribute contains a list of space-separated strings, each corresponding to the id attribute of the <th> elements that apply to this element

	[]byte("height"), // [Deprecated since HTML4] [Obsolete since HTML5]

	[]byte("rowspan"), // [OK] Number of rows that the cell is to span
	[]byte("scope"),   // [OK] This enumerated attribute defines the cells that the header (defined in the <th>) element relates to [NOT OK in <td>]

	[]byte("valign"), // [Obsolete since HTML5]
	[]byte("width"),  // [Deprecated since HTML4] [Obsolete since HTML5]
)

// TableTdCellAttributeFilter defines attribute names which table <td> cells can have.
var TableTdCellAttributeFilter = html.GlobalAttributeFilter.Extend(
	[]byte("abbr"),    // [Obsolete since HTML5] [OK in <th>]
	[]byte("align"),   // [Obsolete since HTML5]
	[]byte("axis"),    // [Obsolete since HTML5]
	[]byte("bgcolor"), // [Not Standardized]
	[]byte("char"),    // [Obsolete since HTML5]
	[]byte("charoff"), // [Obsolete since HTML5]

	[]byte("colspan"), // [OK] Number of columns that the cell is to span
	[]byte("headers"), // [OK] This attribute contains a list of space-separated strings, each corresponding to the id attribute of the <th> elements that apply to this element

	[]byte("height"), // [Deprecated since HTML4] [Obsolete since HTML5]

	[]byte("rowspan"), // [OK] Number of rows that the cell is to span

	[]byte("scope"),  // [Obsolete since HTML5] [OK in <th>]
	[]byte("valign"), // [Obsolete since HTML5]
	[]byte("width"),  // [Deprecated since HTML4] [Obsolete since HTML5]
)

func (r *TableHTMLRenderer) renderTableCell(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	n := node.(*ast.TableCell)
	tag := "td"
	if n.Parent().Kind() == ast.KindTableHeader {
		tag = "th"
	}
	if entering {
		align := ""
		if n.Alignment != ast.AlignNone {
			if _, ok := n.AttributeString("align"); !ok { // Skip align render if overridden
				// TODO: "align" is deprecated. style="text-align:%s" instead?
				align = fmt.Sprintf(` align="%s"`, n.Alignment.String())
			}
		}
		fmt.Fprintf(w, "<%s", tag)
		if n.Attributes() != nil {
			if tag == "td" {
				html.RenderAttributes(w, n, TableTdCellAttributeFilter) // <td>
			} else {
				html.RenderAttributes(w, n, TableThCellAttributeFilter) // <th>
			}
		}
		fmt.Fprintf(w, "%s>", align)
	} else {
		fmt.Fprintf(w, "</%s>\n", tag)
	}
	return gast.WalkContinue, nil
}

type table struct {
}

// Table is an extension that allow you to use GFM tables .
var Table = &table{}

func (e *table) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithParagraphTransformers(
		util.Prioritized(NewTableParagraphTransformer(), 200),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewTableHTMLRenderer(), 500),
	))
}
