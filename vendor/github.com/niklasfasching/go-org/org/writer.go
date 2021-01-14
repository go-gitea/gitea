package org

import "fmt"

// Writer is the interface that is used to export a parsed document into a new format. See Document.Write().
type Writer interface {
	Before(*Document) // Before is called before any nodes are passed to the writer.
	After(*Document)  // After is called after all nodes have been passed to the writer.
	String() string   // String is called at the very end to retrieve the final output.

	WriterWithExtensions() Writer
	WriteNodesAsString(...Node) string

	WriteKeyword(Keyword)
	WriteInclude(Include)
	WriteComment(Comment)
	WriteNodeWithMeta(NodeWithMeta)
	WriteNodeWithName(NodeWithName)
	WriteHeadline(Headline)
	WriteBlock(Block)
	WriteResult(Result)
	WriteInlineBlock(InlineBlock)
	WriteExample(Example)
	WriteDrawer(Drawer)
	WritePropertyDrawer(PropertyDrawer)
	WriteList(List)
	WriteListItem(ListItem)
	WriteDescriptiveListItem(DescriptiveListItem)
	WriteTable(Table)
	WriteHorizontalRule(HorizontalRule)
	WriteParagraph(Paragraph)
	WriteText(Text)
	WriteEmphasis(Emphasis)
	WriteLatexFragment(LatexFragment)
	WriteStatisticToken(StatisticToken)
	WriteExplicitLineBreak(ExplicitLineBreak)
	WriteLineBreak(LineBreak)
	WriteRegularLink(RegularLink)
	WriteMacro(Macro)
	WriteTimestamp(Timestamp)
	WriteFootnoteLink(FootnoteLink)
	WriteFootnoteDefinition(FootnoteDefinition)
}

func WriteNodes(w Writer, nodes ...Node) {
	w = w.WriterWithExtensions()
	for _, n := range nodes {
		switch n := n.(type) {
		case Keyword:
			w.WriteKeyword(n)
		case Include:
			w.WriteInclude(n)
		case Comment:
			w.WriteComment(n)
		case NodeWithMeta:
			w.WriteNodeWithMeta(n)
		case NodeWithName:
			w.WriteNodeWithName(n)
		case Headline:
			w.WriteHeadline(n)
		case Block:
			w.WriteBlock(n)
		case Result:
			w.WriteResult(n)
		case InlineBlock:
			w.WriteInlineBlock(n)
		case Example:
			w.WriteExample(n)
		case Drawer:
			w.WriteDrawer(n)
		case PropertyDrawer:
			w.WritePropertyDrawer(n)
		case List:
			w.WriteList(n)
		case ListItem:
			w.WriteListItem(n)
		case DescriptiveListItem:
			w.WriteDescriptiveListItem(n)
		case Table:
			w.WriteTable(n)
		case HorizontalRule:
			w.WriteHorizontalRule(n)
		case Paragraph:
			w.WriteParagraph(n)
		case Text:
			w.WriteText(n)
		case Emphasis:
			w.WriteEmphasis(n)
		case LatexFragment:
			w.WriteLatexFragment(n)
		case StatisticToken:
			w.WriteStatisticToken(n)
		case ExplicitLineBreak:
			w.WriteExplicitLineBreak(n)
		case LineBreak:
			w.WriteLineBreak(n)
		case RegularLink:
			w.WriteRegularLink(n)
		case Macro:
			w.WriteMacro(n)
		case Timestamp:
			w.WriteTimestamp(n)
		case FootnoteLink:
			w.WriteFootnoteLink(n)
		case FootnoteDefinition:
			w.WriteFootnoteDefinition(n)
		default:
			if n != nil {
				panic(fmt.Sprintf("bad node %T %#v", n, n))
			}
		}
	}
}
