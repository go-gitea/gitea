// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"golang.org/x/net/html"
)

func visitNodeImg(ctx *RenderContext, img *html.Node) (next *html.Node) {
	next = img.NextSibling
	for i, attr := range img.Attr {
		if attr.Key != "src" {
			continue
		}

		if IsNonEmptyRelativePath(attr.Val) {
			attr.Val = ctx.RenderHelper.ResolveLink(attr.Val, LinkTypeMedia)

			// By default, the "<img>" tag should also be clickable,
			// because frontend use `<img>` to paste the re-scaled image into the markdown,
			// so it must match the default markdown image behavior.
			hasParentAnchor := false
			for p := img.Parent; p != nil; p = p.Parent {
				if hasParentAnchor = p.Type == html.ElementNode && p.Data == "a"; hasParentAnchor {
					break
				}
			}
			if !hasParentAnchor {
				imgA := &html.Node{Type: html.ElementNode, Data: "a", Attr: []html.Attribute{
					{Key: "href", Val: attr.Val},
					{Key: "target", Val: "_blank"},
				}}
				parent := img.Parent
				imgNext := img.NextSibling
				parent.RemoveChild(img)
				parent.InsertBefore(imgA, imgNext)
				imgA.AppendChild(img)
			}
		}
		attr.Val = camoHandleLink(attr.Val)
		img.Attr[i] = attr
	}
	return next
}

func visitNodeVideo(ctx *RenderContext, node *html.Node) (next *html.Node) {
	next = node.NextSibling
	for i, attr := range node.Attr {
		if attr.Key != "src" {
			continue
		}
		if IsNonEmptyRelativePath(attr.Val) {
			attr.Val = ctx.RenderHelper.ResolveLink(attr.Val, LinkTypeMedia)
		}
		attr.Val = camoHandleLink(attr.Val)
		node.Attr[i] = attr
	}
	return next
}
