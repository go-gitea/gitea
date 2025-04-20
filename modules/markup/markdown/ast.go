// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"html/template"
	"strconv"

	"github.com/yuin/goldmark/ast"
)

// Details is a block that contains Summary and details
type Details struct {
	ast.BaseBlock
}

// Dump implements Node.Dump .
func (n *Details) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// KindDetails is the NodeKind for Details
var KindDetails = ast.NewNodeKind("Details")

// Kind implements Node.Kind.
func (n *Details) Kind() ast.NodeKind {
	return KindDetails
}

// NewDetails returns a new Paragraph node.
func NewDetails() *Details {
	return &Details{}
}

// Summary is a block that contains the summary of details block
type Summary struct {
	ast.BaseBlock
}

// Dump implements Node.Dump .
func (n *Summary) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// KindSummary is the NodeKind for Summary
var KindSummary = ast.NewNodeKind("Summary")

// Kind implements Node.Kind.
func (n *Summary) Kind() ast.NodeKind {
	return KindSummary
}

// NewSummary returns a new Summary node.
func NewSummary() *Summary {
	return &Summary{}
}

// TaskCheckBoxListItem is a block that represents a list item of a markdown block with a checkbox
type TaskCheckBoxListItem struct {
	*ast.ListItem
	IsChecked      bool
	SourcePosition int
}

// KindTaskCheckBoxListItem is the NodeKind for TaskCheckBoxListItem
var KindTaskCheckBoxListItem = ast.NewNodeKind("TaskCheckBoxListItem")

// Dump implements Node.Dump .
func (n *TaskCheckBoxListItem) Dump(source []byte, level int) {
	m := map[string]string{}
	m["IsChecked"] = strconv.FormatBool(n.IsChecked)
	m["SourcePosition"] = strconv.FormatInt(int64(n.SourcePosition), 10)
	ast.DumpHelper(n, source, level, m, nil)
}

// Kind implements Node.Kind.
func (n *TaskCheckBoxListItem) Kind() ast.NodeKind {
	return KindTaskCheckBoxListItem
}

// NewTaskCheckBoxListItem returns a new TaskCheckBoxListItem node.
func NewTaskCheckBoxListItem(listItem *ast.ListItem) *TaskCheckBoxListItem {
	return &TaskCheckBoxListItem{
		ListItem: listItem,
	}
}

// Icon is an inline for a Fomantic UI icon
type Icon struct {
	ast.BaseInline
	Name []byte
}

// ColorPreview is an inline for a color preview
type ColorPreview struct {
	ast.BaseInline
	Color []byte
}

// Dump implements Node.Dump.
func (n *ColorPreview) Dump(source []byte, level int) {
	m := map[string]string{}
	m["Color"] = string(n.Color)
	ast.DumpHelper(n, source, level, m, nil)
}

// KindColorPreview is the NodeKind for ColorPreview
var KindColorPreview = ast.NewNodeKind("ColorPreview")

// Kind implements Node.Kind.
func (n *ColorPreview) Kind() ast.NodeKind {
	return KindColorPreview
}

// NewColorPreview returns a new Span node.
func NewColorPreview(color []byte) *ColorPreview {
	return &ColorPreview{
		BaseInline: ast.BaseInline{},
		Color:      color,
	}
}

// Attention is an inline for an attention
type Attention struct {
	ast.BaseInline
	AttentionType string
}

// Dump implements Node.Dump.
func (n *Attention) Dump(source []byte, level int) {
	m := map[string]string{}
	m["AttentionType"] = n.AttentionType
	ast.DumpHelper(n, source, level, m, nil)
}

// KindAttention is the NodeKind for Attention
var KindAttention = ast.NewNodeKind("Attention")

// Kind implements Node.Kind.
func (n *Attention) Kind() ast.NodeKind {
	return KindAttention
}

// NewAttention returns a new Attention node.
func NewAttention(attentionType string) *Attention {
	return &Attention{
		BaseInline:    ast.BaseInline{},
		AttentionType: attentionType,
	}
}

var KindRawHTML = ast.NewNodeKind("RawHTML")

type RawHTML struct {
	ast.BaseBlock
	rawHTML template.HTML
}

func (n *RawHTML) Dump(source []byte, level int) {
	m := map[string]string{}
	m["RawHTML"] = string(n.rawHTML)
	ast.DumpHelper(n, source, level, m, nil)
}

func (n *RawHTML) Kind() ast.NodeKind {
	return KindRawHTML
}

func NewRawHTML(rawHTML template.HTML) *RawHTML {
	return &RawHTML{rawHTML: rawHTML}
}
