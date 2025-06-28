// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"

	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/setting"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func createEmoji(ctx *RenderContext, content, name string) *html.Node {
	span := &html.Node{
		Type: html.ElementNode,
		Data: atom.Span.String(),
		Attr: []html.Attribute{},
	}
	span.Attr = append(span.Attr, ctx.RenderInternal.NodeSafeAttr("class", "emoji"))
	if name != "" {
		span.Attr = append(span.Attr, html.Attribute{Key: "aria-label", Val: name})
	}

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}

	span.AppendChild(text)
	return span
}

func createCustomEmoji(ctx *RenderContext, alias string) *html.Node {
	span := &html.Node{
		Type: html.ElementNode,
		Data: atom.Span.String(),
		Attr: []html.Attribute{},
	}
	span.Attr = append(span.Attr, ctx.RenderInternal.NodeSafeAttr("class", "emoji"))
	span.Attr = append(span.Attr, html.Attribute{Key: "aria-label", Val: alias})

	img := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Img,
		Data:     "img",
		Attr:     []html.Attribute{},
	}
	img.Attr = append(img.Attr, html.Attribute{Key: "alt", Val: ":" + alias + ":"})
	img.Attr = append(img.Attr, html.Attribute{Key: "src", Val: setting.StaticURLPrefix + "/assets/img/emoji/" + alias + ".png"})

	span.AppendChild(img)
	return span
}

// emojiShortCodeProcessor for rendering text like :smile: into emoji
func emojiShortCodeProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := globalVars().emojiShortCodeRegex.FindStringSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[0] += start
		m[1] += start

		start = m[1]

		alias := node.Data[m[0]:m[1]]
		alias = strings.ReplaceAll(alias, ":", "")
		converted := emoji.FromAlias(alias)
		if converted == nil {
			// check if this is a custom reaction
			if _, exist := setting.UI.CustomEmojisMap[alias]; exist {
				replaceContent(node, m[0], m[1], createCustomEmoji(ctx, alias))
				node = node.NextSibling.NextSibling
				start = 0
				continue
			}
			continue
		}

		replaceContent(node, m[0], m[1], createEmoji(ctx, converted.Emoji, converted.Description))
		node = node.NextSibling.NextSibling
		start = 0
	}
}

// emoji processor to match emoji and add emoji class
func emojiProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := emoji.FindEmojiSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[0] += start
		m[1] += start

		codepoint := node.Data[m[0]:m[1]]
		start = m[1]
		val := emoji.FromCode(codepoint)
		if val != nil {
			replaceContent(node, m[0], m[1], createEmoji(ctx, codepoint, val.Description))
			node = node.NextSibling.NextSibling
			start = 0
		}
	}
}
