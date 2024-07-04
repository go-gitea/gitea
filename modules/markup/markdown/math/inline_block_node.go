// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package math

import (
	"github.com/yuin/goldmark/ast"
)

// InlineBlock represents inline math e.g. $$...$$
type InlineBlock struct {
	Inline
}

// InlineBlock implements InlineBlock.
func (n *InlineBlock) InlineBlock() {}

// KindInlineBlock is the kind for math inline block
var KindInlineBlock = ast.NewNodeKind("MathInlineBlock")

// Kind returns KindInlineBlock
func (n *InlineBlock) Kind() ast.NodeKind {
	return KindInlineBlock
}

// NewInlineBlock creates a new ast math inline block node
func NewInlineBlock() *InlineBlock {
	return &InlineBlock{
		Inline{},
	}
}
