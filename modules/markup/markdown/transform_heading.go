// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"

	"code.gitea.io/gitea/modules/markup"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

func (g *ASTTransformer) transformHeading(_ *markup.RenderContext, v *ast.Heading, reader text.Reader, tocList *[]markup.Header) {
	for _, attr := range v.Attributes() {
		if _, ok := attr.Value.([]byte); !ok {
			v.SetAttribute(attr.Name, []byte(fmt.Sprintf("%v", attr.Value)))
		}
	}
	txt := v.Text(reader.Source())
	header := markup.Header{
		Text:  util.BytesToReadOnlyString(txt),
		Level: v.Level,
	}
	if id, found := v.AttributeString("id"); found {
		header.ID = util.BytesToReadOnlyString(id.([]byte))
	}
	*tocList = append(*tocList, header)
	g.applyElementDir(v)
}
