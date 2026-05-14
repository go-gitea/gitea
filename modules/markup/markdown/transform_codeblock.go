// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func (g *ASTTransformer) transformFencedCodeblock(v *ast.FencedCodeBlock, reader text.Reader) {
	// * Some engines support a meta syntax for appending the filename after the language, separated by a colon
	//   * https://www.glukhov.org/documentation-tools/markdown/markdown-codeblocks/
	// * Some engines support additional "options" after the language, separated by a space or comma: ```rust,ignore```
	//   * https://docs.readme.com/rdmd/docs/code-blocks
	//   * https://next-book.vercel.app/reference/fencedcode
	if v.Info == nil {
		return
	}
	info := v.Info.Segment.Value(reader.Source())
	newEnd := -1
	for i, b := range info {
		if b == ' ' || b == ',' || b == ':' {
			newEnd = i
			break
		}
	}
	if newEnd != -1 {
		start := v.Info.Segment.Start
		v.Info = ast.NewTextSegment(text.NewSegment(start, start+newEnd))
	}
}
