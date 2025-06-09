// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"

	"golang.org/x/net/html"
)

func isAnchorIDUserContent(s string) bool {
	// blackfridayExtRegex is for blackfriday extensions create IDs like fn:user-content-footnote
	// old logic: blackfridayExtRegex = regexp.MustCompile(`[^:]*:user-content-`)
	return strings.HasPrefix(s, "user-content-") || strings.Contains(s, ":user-content-")
}

func isAnchorIDFootnote(s string) bool {
	return strings.HasPrefix(s, "fnref:user-content-") || strings.HasPrefix(s, "fn:user-content-")
}

func isAnchorHrefFootnote(s string) bool {
	return strings.HasPrefix(s, "#fnref:user-content-") || strings.HasPrefix(s, "#fn:user-content-")
}

func processNodeAttrID(node *html.Node) {
	// Add user-content- to IDs and "#" links if they don't already have them,
	// and convert the link href to a relative link to the host root
	for idx, attr := range node.Attr {
		if attr.Key == "id" {
			if !isAnchorIDUserContent(attr.Val) {
				node.Attr[idx].Val = "user-content-" + attr.Val
			}
		}
	}
}

func processFootnoteNode(ctx *RenderContext, node *html.Node) {
	for idx, attr := range node.Attr {
		if (attr.Key == "id" && isAnchorIDFootnote(attr.Val)) ||
			(attr.Key == "href" && isAnchorHrefFootnote(attr.Val)) {
			if footnoteContextID := ctx.RenderOptions.Metas["footnoteContextId"]; footnoteContextID != "" {
				node.Attr[idx].Val = attr.Val + "-" + footnoteContextID
			}
			continue
		}
	}
}

func processNodeA(ctx *RenderContext, node *html.Node) {
	for idx, attr := range node.Attr {
		if attr.Key == "href" {
			if anchorID, ok := strings.CutPrefix(attr.Val, "#"); ok {
				if !isAnchorIDUserContent(attr.Val) {
					node.Attr[idx].Val = "#user-content-" + anchorID
				}
			} else {
				node.Attr[idx].Val = ctx.RenderHelper.ResolveLink(attr.Val, LinkTypeDefault)
			}
		}
	}
}

func visitNodeImg(ctx *RenderContext, img *html.Node) (next *html.Node) {
	next = img.NextSibling
	for i, imgAttr := range img.Attr {
		if imgAttr.Key != "src" {
			continue
		}

		imgSrcOrigin := imgAttr.Val
		isLinkable := imgSrcOrigin != "" && !strings.HasPrefix(imgSrcOrigin, "data:")

		// By default, the "<img>" tag should also be clickable,
		// because frontend use `<img>` to paste the re-scaled image into the markdown,
		// so it must match the default markdown image behavior.
		cnt := 0
		for p := img.Parent; isLinkable && p != nil && cnt < 2; p = p.Parent {
			if hasParentAnchor := p.Type == html.ElementNode && p.Data == "a"; hasParentAnchor {
				isLinkable = false
				break
			}
			cnt++
		}
		if isLinkable {
			wrapper := &html.Node{Type: html.ElementNode, Data: "a", Attr: []html.Attribute{
				{Key: "href", Val: ctx.RenderHelper.ResolveLink(imgSrcOrigin, LinkTypeDefault)},
				{Key: "target", Val: "_blank"},
			}}
			parent := img.Parent
			imgNext := img.NextSibling
			parent.RemoveChild(img)
			parent.InsertBefore(wrapper, imgNext)
			wrapper.AppendChild(img)
		}

		imgAttr.Val = ctx.RenderHelper.ResolveLink(imgSrcOrigin, LinkTypeMedia)
		imgAttr.Val = camoHandleLink(imgAttr.Val)
		img.Attr[i] = imgAttr
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
