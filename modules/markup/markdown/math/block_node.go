// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package math

import "github.com/yuin/goldmark/ast"

// Block represents a display math block e.g. $$...$$ or \[...\]
type Block struct {
	ast.BaseBlock
	Dollars bool
	Indent  int
	Closed  bool
	Inline  bool
}

// KindBlock is the node kind for math blocks
var KindBlock = ast.NewNodeKind("MathBlock")

// NewBlock creates a new math Block
func NewBlock(dollars bool, indent int) *Block {
	return &Block{
		Dollars: dollars,
		Indent:  indent,
	}
}

// Dump dumps the block to a string
func (n *Block) Dump(source []byte, level int) {
	m := map[string]string{}
	ast.DumpHelper(n, source, level, m, nil)
}

// Kind returns KindBlock for math Blocks
func (n *Block) Kind() ast.NodeKind {
	return KindBlock
}

// IsRaw returns true as this block should not be processed further
func (n *Block) IsRaw() bool {
	return true
}
