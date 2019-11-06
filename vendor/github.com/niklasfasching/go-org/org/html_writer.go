package org

import (
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"unicode"

	h "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTMLWriter exports an org document into a html document.
type HTMLWriter struct {
	ExtendingWriter    Writer
	HighlightCodeBlock func(source, lang string) string

	strings.Builder
	document   *Document
	htmlEscape bool
	log        *log.Logger
	footnotes  *footnotes
}

type footnotes struct {
	mapping map[string]int
	list    []*FootnoteDefinition
}

var emphasisTags = map[string][]string{
	"/":   []string{"<em>", "</em>"},
	"*":   []string{"<strong>", "</strong>"},
	"+":   []string{"<del>", "</del>"},
	"~":   []string{"<code>", "</code>"},
	"=":   []string{`<code class="verbatim">`, "</code>"},
	"_":   []string{`<span style="text-decoration: underline;">`, "</span>"},
	"_{}": []string{"<sub>", "</sub>"},
	"^{}": []string{"<sup>", "</sup>"},
}

var listTags = map[string][]string{
	"unordered":   []string{"<ul>", "</ul>"},
	"ordered":     []string{"<ol>", "</ol>"},
	"descriptive": []string{"<dl>", "</dl>"},
}

var listItemStatuses = map[string]string{
	" ": "unchecked",
	"-": "indeterminate",
	"X": "checked",
}

var cleanHeadlineTitleForHTMLAnchorRegexp = regexp.MustCompile(`</?a[^>]*>`) // nested a tags are not valid HTML

func NewHTMLWriter() *HTMLWriter {
	defaultConfig := New()
	return &HTMLWriter{
		document:   &Document{Configuration: defaultConfig},
		log:        defaultConfig.Log,
		htmlEscape: true,
		HighlightCodeBlock: func(source, lang string) string {
			return fmt.Sprintf("<div class=\"highlight\">\n<pre>\n%s\n</pre>\n</div>", html.EscapeString(source))
		},
		footnotes: &footnotes{
			mapping: map[string]int{},
		},
	}
}

func (w *HTMLWriter) WriteNodesAsString(nodes ...Node) string {
	original := w.Builder
	w.Builder = strings.Builder{}
	WriteNodes(w, nodes...)
	out := w.String()
	w.Builder = original
	return out
}

func (w *HTMLWriter) WriterWithExtensions() Writer {
	if w.ExtendingWriter != nil {
		return w.ExtendingWriter
	}
	return w
}

func (w *HTMLWriter) Before(d *Document) {
	w.document = d
	w.log = d.Log
	w.WriteOutline(d)
}

func (w *HTMLWriter) After(d *Document) {
	w.WriteFootnotes(d)
}

func (w *HTMLWriter) WriteComment(Comment)               {}
func (w *HTMLWriter) WritePropertyDrawer(PropertyDrawer) {}

func (w *HTMLWriter) WriteBlock(b Block) {
	content := ""
	if isRawTextBlock(b.Name) {
		builder, htmlEscape := w.Builder, w.htmlEscape
		w.Builder, w.htmlEscape = strings.Builder{}, false
		WriteNodes(w, b.Children...)
		out := w.String()
		w.Builder, w.htmlEscape = builder, htmlEscape
		content = strings.TrimRightFunc(out, unicode.IsSpace)
	} else {
		content = w.WriteNodesAsString(b.Children...)
	}
	switch name := b.Name; {
	case name == "SRC":
		lang := "text"
		if len(b.Parameters) >= 1 {
			lang = strings.ToLower(b.Parameters[0])
		}
		content = w.HighlightCodeBlock(content, lang)
		w.WriteString(fmt.Sprintf("<div class=\"src src-%s\">\n%s\n</div>\n", lang, content))
	case name == "EXAMPLE":
		w.WriteString(`<pre class="example">` + "\n" + content + "\n</pre>\n")
	case name == "EXPORT" && len(b.Parameters) >= 1 && strings.ToLower(b.Parameters[0]) == "html":
		w.WriteString(content + "\n")
	case name == "QUOTE":
		w.WriteString("<blockquote>\n" + content + "</blockquote>\n")
	case name == "CENTER":
		w.WriteString(`<div class="center-block" style="text-align: center; margin-left: auto; margin-right: auto;">` + "\n")
		w.WriteString(content + "</div>\n")
	default:
		w.WriteString(fmt.Sprintf(`<div class="%s-block">`, strings.ToLower(b.Name)) + "\n")
		w.WriteString(content + "</div>\n")
	}
}

func (w *HTMLWriter) WriteDrawer(d Drawer) {
	WriteNodes(w, d.Children...)
}

func (w *HTMLWriter) WriteKeyword(k Keyword) {
	if k.Key == "HTML" {
		w.WriteString(k.Value + "\n")
	}
}

func (w *HTMLWriter) WriteInclude(i Include) {
	WriteNodes(w, i.Resolve())
}

func (w *HTMLWriter) WriteFootnoteDefinition(f FootnoteDefinition) {
	w.footnotes.updateDefinition(f)
}

func (w *HTMLWriter) WriteFootnotes(d *Document) {
	if !w.document.GetOption("f") || len(w.footnotes.list) == 0 {
		return
	}
	w.WriteString(`<div class="footnotes">` + "\n")
	w.WriteString(`<hr class="footnotes-separatator">` + "\n")
	w.WriteString(`<div class="footnote-definitions">` + "\n")
	for i, definition := range w.footnotes.list {
		id := i + 1
		if definition == nil {
			name := ""
			for k, v := range w.footnotes.mapping {
				if v == i {
					name = k
				}
			}
			w.log.Printf("Missing footnote definition for [fn:%s] (#%d)", name, id)
			continue
		}
		w.WriteString(`<div class="footnote-definition">` + "\n")
		w.WriteString(fmt.Sprintf(`<sup id="footnote-%d"><a href="#footnote-reference-%d">%d</a></sup>`, id, id, id) + "\n")
		w.WriteString(`<div class="footnote-body">` + "\n")
		WriteNodes(w, definition.Children...)
		w.WriteString("</div>\n</div>\n")
	}
	w.WriteString("</div>\n</div>\n")
}

func (w *HTMLWriter) WriteOutline(d *Document) {
	if w.document.GetOption("toc") && len(d.Outline.Children) != 0 {
		w.WriteString("<nav>\n<ul>\n")
		for _, section := range d.Outline.Children {
			w.writeSection(section)
		}
		w.WriteString("</ul>\n</nav>\n")
	}
}

func (w *HTMLWriter) writeSection(section *Section) {
	// NOTE: To satisfy hugo ExtractTOC() check we cannot use `<li>\n` here. Doesn't really matter, just a note.
	w.WriteString("<li>")
	h := section.Headline
	title := cleanHeadlineTitleForHTMLAnchorRegexp.ReplaceAllString(w.WriteNodesAsString(h.Title...), "")
	w.WriteString(fmt.Sprintf("<a href=\"#%s\">%s</a>\n", h.ID(), title))
	if len(section.Children) != 0 {
		w.WriteString("<ul>\n")
		for _, section := range section.Children {
			w.writeSection(section)
		}
		w.WriteString("</ul>\n")
	}
	w.WriteString("</li>\n")
}

func (w *HTMLWriter) WriteHeadline(h Headline) {
	for _, excludeTag := range strings.Fields(w.document.Get("EXCLUDE_TAGS")) {
		for _, tag := range h.Tags {
			if excludeTag == tag {
				return
			}
		}
	}

	w.WriteString(fmt.Sprintf(`<h%d id="%s">`, h.Lvl, h.ID()) + "\n")
	if w.document.GetOption("todo") && h.Status != "" {
		w.WriteString(fmt.Sprintf(`<span class="todo">%s</span>`, h.Status) + "\n")
	}
	if w.document.GetOption("pri") && h.Priority != "" {
		w.WriteString(fmt.Sprintf(`<span class="priority">[%s]</span>`, h.Priority) + "\n")
	}

	WriteNodes(w, h.Title...)
	if w.document.GetOption("tags") && len(h.Tags) != 0 {
		tags := make([]string, len(h.Tags))
		for i, tag := range h.Tags {
			tags[i] = fmt.Sprintf(`<span>%s</span>`, tag)
		}
		w.WriteString("&#xa0;&#xa0;&#xa0;")
		w.WriteString(fmt.Sprintf(`<span class="tags">%s</span>`, strings.Join(tags, "&#xa0;")))
	}
	w.WriteString(fmt.Sprintf("\n</h%d>\n", h.Lvl))
	WriteNodes(w, h.Children...)
}

func (w *HTMLWriter) WriteText(t Text) {
	if !w.htmlEscape {
		w.WriteString(t.Content)
	} else if !w.document.GetOption("e") || t.IsRaw {
		w.WriteString(html.EscapeString(t.Content))
	} else {
		w.WriteString(html.EscapeString(htmlEntityReplacer.Replace(t.Content)))
	}
}

func (w *HTMLWriter) WriteEmphasis(e Emphasis) {
	tags, ok := emphasisTags[e.Kind]
	if !ok {
		panic(fmt.Sprintf("bad emphasis %#v", e))
	}
	w.WriteString(tags[0])
	WriteNodes(w, e.Content...)
	w.WriteString(tags[1])
}

func (w *HTMLWriter) WriteLatexFragment(l LatexFragment) {
	w.WriteString(l.OpeningPair)
	WriteNodes(w, l.Content...)
	w.WriteString(l.ClosingPair)
}

func (w *HTMLWriter) WriteStatisticToken(s StatisticToken) {
	w.WriteString(fmt.Sprintf(`<code class="statistic">[%s]</code>`, s.Content))
}

func (w *HTMLWriter) WriteLineBreak(l LineBreak) {
	w.WriteString(strings.Repeat("\n", l.Count))
}

func (w *HTMLWriter) WriteExplicitLineBreak(l ExplicitLineBreak) {
	w.WriteString("<br>\n")
}

func (w *HTMLWriter) WriteFootnoteLink(l FootnoteLink) {
	if !w.document.GetOption("f") {
		return
	}
	i := w.footnotes.add(l)
	id := i + 1
	w.WriteString(fmt.Sprintf(`<sup class="footnote-reference"><a id="footnote-reference-%d" href="#footnote-%d">%d</a></sup>`, id, id, id))
}

func (w *HTMLWriter) WriteTimestamp(t Timestamp) {
	if !w.document.GetOption("<") {
		return
	}
	w.WriteString(`<span class="timestamp">&lt;`)
	if t.IsDate {
		w.WriteString(t.Time.Format(datestampFormat))
	} else {
		w.WriteString(t.Time.Format(timestampFormat))
	}
	if t.Interval != "" {
		w.WriteString(" " + t.Interval)
	}
	w.WriteString(`&gt;</span>`)
}

func (w *HTMLWriter) WriteRegularLink(l RegularLink) {
	url := html.EscapeString(l.URL)
	if l.Protocol == "file" {
		url = url[len("file:"):]
	}
	description := url
	if l.Description != nil {
		description = w.WriteNodesAsString(l.Description...)
	}
	switch l.Kind() {
	case "image":
		w.WriteString(fmt.Sprintf(`<img src="%s" alt="%s" title="%s" />`, url, description, description))
	case "video":
		w.WriteString(fmt.Sprintf(`<video src="%s" title="%s">%s</video>`, url, description, description))
	default:
		w.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, url, description))
	}
}

func (w *HTMLWriter) WriteList(l List) {
	tags, ok := listTags[l.Kind]
	if !ok {
		panic(fmt.Sprintf("bad list kind %#v", l))
	}
	w.WriteString(tags[0] + "\n")
	WriteNodes(w, l.Items...)
	w.WriteString(tags[1] + "\n")
}

func (w *HTMLWriter) WriteListItem(li ListItem) {
	if li.Status != "" {
		w.WriteString(fmt.Sprintf("<li class=\"%s\">\n", listItemStatuses[li.Status]))
	} else {
		w.WriteString("<li>\n")
	}
	WriteNodes(w, li.Children...)
	w.WriteString("</li>\n")
}

func (w *HTMLWriter) WriteDescriptiveListItem(di DescriptiveListItem) {
	if di.Status != "" {
		w.WriteString(fmt.Sprintf("<dt class=\"%s\">\n", listItemStatuses[di.Status]))
	} else {
		w.WriteString("<dt>\n")
	}

	if len(di.Term) != 0 {
		WriteNodes(w, di.Term...)
	} else {
		w.WriteString("?")
	}
	w.WriteString("\n</dt>\n")
	w.WriteString("<dd>\n")
	WriteNodes(w, di.Details...)
	w.WriteString("</dd>\n")
}

func (w *HTMLWriter) WriteParagraph(p Paragraph) {
	if len(p.Children) == 0 {
		return
	}
	w.WriteString("<p>")
	if _, ok := p.Children[0].(LineBreak); !ok {
		w.WriteString("\n")
	}
	WriteNodes(w, p.Children...)
	w.WriteString("\n</p>\n")
}

func (w *HTMLWriter) WriteExample(e Example) {
	w.WriteString(`<pre class="example">` + "\n")
	if len(e.Children) != 0 {
		for _, n := range e.Children {
			WriteNodes(w, n)
			w.WriteString("\n")
		}
	}
	w.WriteString("</pre>\n")
}

func (w *HTMLWriter) WriteHorizontalRule(h HorizontalRule) {
	w.WriteString("<hr>\n")
}

func (w *HTMLWriter) WriteNodeWithMeta(n NodeWithMeta) {
	out := w.WriteNodesAsString(n.Node)
	if p, ok := n.Node.(Paragraph); ok {
		if len(p.Children) == 1 && isImageOrVideoLink(p.Children[0]) {
			out = w.WriteNodesAsString(p.Children[0])
		}
	}
	for _, attributes := range n.Meta.HTMLAttributes {
		out = w.withHTMLAttributes(out, attributes...) + "\n"
	}
	if len(n.Meta.Caption) != 0 {
		caption := ""
		for i, ns := range n.Meta.Caption {
			if i != 0 {
				caption += " "
			}
			caption += w.WriteNodesAsString(ns...)
		}
		out = fmt.Sprintf("<figure>\n%s<figcaption>\n%s\n</figcaption>\n</figure>\n", out, caption)
	}
	w.WriteString(out)
}

func (w *HTMLWriter) WriteNodeWithName(n NodeWithName) {
	WriteNodes(w, n.Node)
}

func (w *HTMLWriter) WriteTable(t Table) {
	w.WriteString("<table>\n")
	beforeFirstContentRow := true
	for i, row := range t.Rows {
		if row.IsSpecial || len(row.Columns) == 0 {
			continue
		}
		if beforeFirstContentRow {
			beforeFirstContentRow = false
			if i+1 < len(t.Rows) && len(t.Rows[i+1].Columns) == 0 {
				w.WriteString("<thead>\n")
				w.writeTableColumns(row.Columns, "th")
				w.WriteString("</thead>\n<tbody>\n")
				continue
			} else {
				w.WriteString("<tbody>\n")
			}
		}
		w.writeTableColumns(row.Columns, "td")
	}
	w.WriteString("</tbody>\n</table>\n")
}

func (w *HTMLWriter) writeTableColumns(columns []Column, tag string) {
	w.WriteString("<tr>\n")
	for _, column := range columns {
		if column.Align == "" {
			w.WriteString(fmt.Sprintf("<%s>", tag))
		} else {
			w.WriteString(fmt.Sprintf(`<%s class="align-%s">`, tag, column.Align))
		}
		WriteNodes(w, column.Children...)
		w.WriteString(fmt.Sprintf("</%s>\n", tag))
	}
	w.WriteString("</tr>\n")
}

func (w *HTMLWriter) withHTMLAttributes(input string, kvs ...string) string {
	if len(kvs)%2 != 0 {
		w.log.Printf("withHTMLAttributes: Len of kvs must be even: %#v", kvs)
		return input
	}
	context := &h.Node{Type: h.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := h.ParseFragment(strings.NewReader(strings.TrimSpace(input)), context)
	if err != nil || len(nodes) != 1 {
		w.log.Printf("withHTMLAttributes: Could not extend attributes of %s: %v (%s)", input, nodes, err)
		return input
	}
	out, node := strings.Builder{}, nodes[0]
	for i := 0; i < len(kvs)-1; i += 2 {
		node.Attr = setHTMLAttribute(node.Attr, strings.TrimPrefix(kvs[i], ":"), kvs[i+1])
	}
	err = h.Render(&out, nodes[0])
	if err != nil {
		w.log.Printf("withHTMLAttributes: Could not extend attributes of %s: %v (%s)", input, node, err)
		return input
	}
	return out.String()
}

func setHTMLAttribute(attributes []h.Attribute, k, v string) []h.Attribute {
	for i, a := range attributes {
		if strings.ToLower(a.Key) == strings.ToLower(k) {
			switch strings.ToLower(k) {
			case "class", "style":
				attributes[i].Val += " " + v
			default:
				attributes[i].Val = v
			}
			return attributes
		}
	}
	return append(attributes, h.Attribute{Namespace: "", Key: k, Val: v})
}

func (fs *footnotes) add(f FootnoteLink) int {
	if i, ok := fs.mapping[f.Name]; ok && f.Name != "" {
		return i
	}
	fs.list = append(fs.list, f.Definition)
	i := len(fs.list) - 1
	if f.Name != "" {
		fs.mapping[f.Name] = i
	}
	return i
}

func (fs *footnotes) updateDefinition(f FootnoteDefinition) {
	if i, ok := fs.mapping[f.Name]; ok {
		fs.list[i] = &f
	}
}
