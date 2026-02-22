// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"

	"code.gitea.io/gitea/modules/markup/common"

	"golang.org/x/net/html"
)

func isAnchorIDUserContent(s string) bool {
	// blackfridayExtRegex is for blackfriday extensions create IDs like fn:user-content-footnote
	// old logic: blackfridayExtRegex = regexp.MustCompile(`[^:]*:user-content-`)
	return strings.HasPrefix(s, "user-content-") || strings.Contains(s, ":user-content-") || isAnchorIDFootnote(s)
}

func isAnchorIDFootnote(s string) bool {
	return strings.HasPrefix(s, "fnref:user-content-") || strings.HasPrefix(s, "fn:user-content-")
}

func isAnchorHrefFootnote(s string) bool {
	return strings.HasPrefix(s, "#fnref:user-content-") || strings.HasPrefix(s, "#fn:user-content-")
}

// isHeadingTag returns true if the node is a heading tag (h1-h6)
func isHeadingTag(node *html.Node) bool {
	return node.Type == html.ElementNode &&
		len(node.Data) == 2 &&
		node.Data[0] == 'h' &&
		node.Data[1] >= '1' && node.Data[1] <= '6'
}

// getNodeText extracts the text content from a node and its children
func getNodeText(node *html.Node, cached **string) string {
	if *cached != nil {
		return **cached
	}
	var text strings.Builder
	var extractText func(*html.Node)
	extractText = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractText(c)
		}
	}
	extractText(node)
	textStr := text.String()
	*cached = &textStr
	return textStr
}

func processNodeHeadingAndID(ctx *RenderContext, node *html.Node) {
	// TODO: handle duplicate IDs, need to track existing IDs in the document
	// Add user-content- to IDs and "#" links if they don't already have them,
	// and convert the link href to a relative link to the host root
	attrIDVal := ""
	for idx, attr := range node.Attr {
		if attr.Key == "id" {
			attrIDVal = attr.Val
			if !isAnchorIDUserContent(attrIDVal) {
				attrIDVal = "user-content-" + attrIDVal
				node.Attr[idx].Val = attrIDVal
			}
		}
	}

	if !isHeadingTag(node) || !ctx.RenderOptions.EnableHeadingIDGeneration {
		return
	}

	// For heading tags (h1-h6) without an id attribute, generate one from the text content.
	// This ensures HTML headings like <h1>Title</h1> get proper permalink anchors
	// matching the behavior of Markdown headings.
	// Only enabled for repository files and wiki pages via EnableHeadingIDGeneration option.
	var nodeTextCached *string
	if attrIDVal == "" {
		nodeText := getNodeText(node, &nodeTextCached)
		if nodeText != "" {
			// Use the same CleanValue function used by Markdown heading ID generation
			attrIDVal = string(common.CleanValue([]byte(nodeText)))
			if attrIDVal != "" {
				attrIDVal = "user-content-" + attrIDVal
				node.Attr = append(node.Attr, html.Attribute{Key: "id", Val: attrIDVal})
			}
		}
	}
	if ctx.TocShowInSection != "" {
		nodeText := getNodeText(node, &nodeTextCached)
		if nodeText != "" && attrIDVal != "" {
			ctx.TocHeadingItems = append(ctx.TocHeadingItems, &TocHeadingItem{
				HeadingLevel: int(node.Data[1] - '0'),
				AnchorID:     attrIDVal,
				InnerText:    nodeText,
			})
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
	attrSrc, hasLazy := "", false
	for i, imgAttr := range img.Attr {
		hasLazy = hasLazy || imgAttr.Key == "loading" && imgAttr.Val == "lazy"
		if imgAttr.Key != "src" {
			attrSrc = imgAttr.Val
			continue
		}

		imgSrcOrigin := imgAttr.Val
		isLinkable := imgSrcOrigin != "" && !strings.HasPrefix(imgSrcOrigin, "data:")

		// By default, the "<img>" tag should also be clickable,
		// because frontend uses `<img>` to paste the re-scaled image into the Markdown,
		// so it must match the default Markdown image behavior.
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
	if !RenderBehaviorForTesting.DisableAdditionalAttributes && !hasLazy && !strings.HasPrefix(attrSrc, "data:") {
		img.Attr = append(img.Attr, html.Attribute{Key: "loading", Val: "lazy"})
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
