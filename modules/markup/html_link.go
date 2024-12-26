// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func shortLinkProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := globalVars().shortLinkPattern.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		content := node.Data[m[2]:m[3]]
		tail := node.Data[m[4]:m[5]]
		props := make(map[string]string)

		// MediaWiki uses [[link|text]], while GitHub uses [[text|link]]
		// It makes page handling terrible, but we prefer GitHub syntax
		// And fall back to MediaWiki only when it is obvious from the look
		// Of text and link contents
		sl := strings.Split(content, "|")
		for _, v := range sl {
			if equalPos := strings.IndexByte(v, '='); equalPos == -1 {
				// There is no equal in this argument; this is a mandatory arg
				if props["name"] == "" {
					if IsFullURLString(v) {
						// If we clearly see it is a link, we save it so

						// But first we need to ensure, that if both mandatory args provided
						// look like links, we stick to GitHub syntax
						if props["link"] != "" {
							props["name"] = props["link"]
						}

						props["link"] = strings.TrimSpace(v)
					} else {
						props["name"] = v
					}
				} else {
					props["link"] = strings.TrimSpace(v)
				}
			} else {
				// There is an equal; optional argument.

				sep := strings.IndexByte(v, '=')
				key, val := v[:sep], html.UnescapeString(v[sep+1:])

				// When parsing HTML, x/net/html will change all quotes which are
				// not used for syntax into UTF-8 quotes. So checking val[0] won't
				// be enough, since that only checks a single byte.
				if len(val) > 1 {
					if (strings.HasPrefix(val, "“") && strings.HasSuffix(val, "”")) ||
						(strings.HasPrefix(val, "‘") && strings.HasSuffix(val, "’")) {
						const lenQuote = len("‘")
						val = val[lenQuote : len(val)-lenQuote]
					} else if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
						(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
						val = val[1 : len(val)-1]
					} else if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "’") {
						const lenQuote = len("‘")
						val = val[1 : len(val)-lenQuote]
					}
				}
				props[key] = val
			}
		}

		var name, link string
		if props["link"] != "" {
			link = props["link"]
		} else if props["name"] != "" {
			link = props["name"]
		}
		if props["title"] != "" {
			name = props["title"]
		} else if props["name"] != "" {
			name = props["name"]
		} else {
			name = link
		}

		name += tail
		image := false
		ext := path.Ext(link)
		switch ext {
		// fast path: empty string, ignore
		case "":
			// leave image as false
		case ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp", ".gif", ".bmp", ".ico", ".svg":
			image = true
		}

		childNode := &html.Node{}
		linkNode := &html.Node{
			FirstChild: childNode,
			LastChild:  childNode,
			Type:       html.ElementNode,
			Data:       "a",
			DataAtom:   atom.A,
		}
		childNode.Parent = linkNode
		absoluteLink := IsFullURLString(link)
		if !absoluteLink {
			if image {
				link = strings.ReplaceAll(link, " ", "+")
			} else {
				// the hacky wiki name encoding: space to "-"
				link = strings.ReplaceAll(link, " ", "-") // FIXME: it should support dashes in the link, eg: "the-dash-support.-"
			}
			if !strings.Contains(link, "/") {
				link = url.PathEscape(link) // FIXME: it doesn't seem right and it might cause double-escaping
			}
		}
		if image {
			link = ctx.RenderHelper.ResolveLink(link, LinkTypeMedia)
			title := props["title"]
			if title == "" {
				title = props["alt"]
			}
			if title == "" {
				title = path.Base(name)
			}
			alt := props["alt"]
			if alt == "" {
				alt = name
			}

			// make the childNode an image - if we can, we also place the alt
			childNode.Type = html.ElementNode
			childNode.Data = "img"
			childNode.DataAtom = atom.Img
			childNode.Attr = []html.Attribute{
				{Key: "src", Val: link},
				{Key: "title", Val: title},
				{Key: "alt", Val: alt},
			}
			if alt == "" {
				childNode.Attr = childNode.Attr[:2]
			}
		} else {
			link = ctx.RenderHelper.ResolveLink(link, LinkTypeDefault)
			childNode.Type = html.TextNode
			childNode.Data = name
		}
		linkNode.Attr = []html.Attribute{{Key: "href", Val: link}}
		replaceContent(node, m[0], m[1], linkNode)
		node = node.NextSibling.NextSibling
	}
}

// linkProcessor creates links for any HTTP or HTTPS URL not captured by
// markdown.
func linkProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := common.GlobalVars().LinkRegex.FindStringIndex(node.Data)
		if m == nil {
			return
		}

		uri := node.Data[m[0]:m[1]]
		remaining := node.Data[m[1]:]
		if util.IsLikelyEllipsisLeftPart(remaining) {
			return
		}
		replaceContent(node, m[0], m[1], createLink(ctx, uri, uri, "" /*link*/))
		node = node.NextSibling.NextSibling
	}
}

// descriptionLinkProcessor creates links for DescriptionHTML
func descriptionLinkProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := common.GlobalVars().LinkRegex.FindStringIndex(node.Data)
		if m == nil {
			return
		}

		uri := node.Data[m[0]:m[1]]
		replaceContent(node, m[0], m[1], createDescriptionLink(uri, uri))
		node = node.NextSibling.NextSibling
	}
}

func createDescriptionLink(href, content string) *html.Node {
	textNode := &html.Node{
		Type: html.TextNode,
		Data: content,
	}
	linkNode := &html.Node{
		FirstChild: textNode,
		LastChild:  textNode,
		Type:       html.ElementNode,
		Data:       "a",
		DataAtom:   atom.A,
		Attr: []html.Attribute{
			{Key: "href", Val: href},
			{Key: "target", Val: "_blank"},
			{Key: "rel", Val: "noopener noreferrer"},
		},
	}
	textNode.Parent = linkNode
	return linkNode
}
