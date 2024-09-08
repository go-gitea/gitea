// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/util"

	"github.com/yuin/goldmark/ast"
)

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
	if p.values.Add(util.UnsafeBytesToString(result)) {
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
	p.values.Add(util.UnsafeBytesToString(value))
}

func newPrefixedIDs() *prefixedIDs {
	return &prefixedIDs{
		values: make(container.Set[string]),
	}
}
