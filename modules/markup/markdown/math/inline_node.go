// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package math

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// Inline struct represents inline math e.g. $...$ or \(...\)
type Inline struct {
	ast.BaseInline
}

// Inline implements Inline.Inline.
func (n *Inline) Inline() {}

// IsBlank returns if this inline node is empty
func (n *Inline) IsBlank(source []byte) bool {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		text := c.(*ast.Text).Segment
		if !util.IsBlank(text.Value(source)) {
			return false
		}
	}
	return true
}

// Dump renders this inline math as debug
func (n *Inline) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// KindInline is the kind for math inline
var KindInline = ast.NewNodeKind("MathInline")

// Kind returns KindInline
func (n *Inline) Kind() ast.NodeKind {
	return KindInline
}

// NewInline creates a new ast math inline node
func NewInline() *Inline {
	return &Inline{
		BaseInline: ast.BaseInline{},
	}
}
