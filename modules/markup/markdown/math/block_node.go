// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package math

import "github.com/yuin/goldmark/ast"

// Block represents a math Block
type Block struct {
	ast.BaseBlock
}

// KindBlock is the node kind for math blocks
var KindBlock = ast.NewNodeKind("MathBlock")

// NewBlock creates a new math Block
func NewBlock() *Block {
	return &Block{}
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
