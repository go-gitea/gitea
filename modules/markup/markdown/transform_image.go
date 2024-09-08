// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"strings"

	"code.gitea.io/gitea/modules/markup"
	giteautil "code.gitea.io/gitea/modules/util"

	"github.com/yuin/goldmark/ast"
)

func (g *ASTTransformer) transformImage(ctx *markup.RenderContext, v *ast.Image) {
	// Images need two things:
	//
	// 1. Their src needs to munged to be a real value
	// 2. If they're not wrapped with a link they need a link wrapper

	// Check if the destination is a real link
	if len(v.Destination) > 0 && !markup.IsFullURLBytes(v.Destination) {
		v.Destination = []byte(giteautil.URLJoin(
			ctx.Links.ResolveMediaLink(ctx.IsWiki),
			strings.TrimLeft(string(v.Destination), "/"),
		))
	}

	parent := v.Parent()
	// Create a link around image only if parent is not already a link
	if _, ok := parent.(*ast.Link); !ok && parent != nil {
		next := v.NextSibling()

		// Create a link wrapper
		wrap := ast.NewLink()
		wrap.Destination = v.Destination
		wrap.Title = v.Title
		wrap.SetAttributeString("target", []byte("_blank"))

		// Duplicate the current image node
		image := ast.NewImage(ast.NewLink())
		image.Destination = v.Destination
		image.Title = v.Title
		for _, attr := range v.Attributes() {
			image.SetAttribute(attr.Name, attr.Value)
		}
		for child := v.FirstChild(); child != nil; {
			next := child.NextSibling()
			image.AppendChild(image, child)
			child = next
		}

		// Append our duplicate image to the wrapper link
		wrap.AppendChild(wrap, image)

		// Wire in the next sibling
		wrap.SetNextSibling(next)

		// Replace the current node with the wrapper link
		parent.ReplaceChild(parent, v, wrap)

		// But most importantly ensure the next sibling is still on the old image too
		v.SetNextSibling(next)
	}
}
