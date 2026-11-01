// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"fmt"
	"net/url"
	"strings"

	"gitea.dev/modules/references"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func createMentionAvatar(ctx *RenderContext, avatarLink string) *html.Node {
	// turn the sub-URL-relative link into a rendered root link, so visitNodeImg resolves it like other app links
	if rel, ok := strings.CutPrefix(avatarLink, setting.AppSubURL+"/"); ok {
		avatarLink = LinkTypeRoot + "/" + rel
	}
	img := &html.Node{
		Type: html.ElementNode,
		Data: atom.Img.String(),
		Attr: []html.Attribute{
			ctx.RenderInternal.NodeSafeAttr("class", "ui avatar mention-avatar"),
			{Key: "src", Val: avatarLink},
			{Key: "alt", Val: ""}, // decorative, the mention text already names the user
			{Key: "width", Val: "20"},
			{Key: "height", Val: "20"},
			{Key: "loading", Val: "lazy"},
		},
	}
	return img
}

func mentionProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	nodeStop := node.NextSibling
	for node != nodeStop {
		found, loc := references.FindFirstMentionBytes(util.UnsafeStringToBytes(node.Data[start:]))
		if !found {
			node = node.NextSibling
			start = 0
			continue
		}
		loc.Start += start
		loc.End += start
		mention := node.Data[loc.Start:loc.End]
		orgLowerTeams, checkOrgTeams := ctx.RenderOptions.Metas["teams"] // in format ",team1,team2,...,team-n,", always lowercase

		if checkOrgTeams && strings.Contains(mention, "/") {
			mentionOrg, teamName, _ := strings.Cut(mention, "/")
			orgName := mentionOrg[1:] // remove the '@' prefix
			if strings.EqualFold(orgName, ctx.RenderOptions.Metas["org"]) && strings.Contains(orgLowerTeams, ","+strings.ToLower(teamName)+",") {
				link := fmt.Sprintf("/:root/org/%s/teams/%s", url.PathEscape(orgName), url.PathEscape(teamName))
				replaceContent(node, loc.Start, loc.End, createLink(ctx, link, mention, "" /*mention*/))
				node = node.NextSibling.NextSibling
				start = 0
				continue
			}
			start = loc.End
			continue
		}
		mentionedUsername := mention[1:]

		var mentionable bool
		var avatarLink string
		if DefaultRenderHelperFuncs != nil && DefaultRenderHelperFuncs.ResolveUserMention != nil {
			mentionable, avatarLink = DefaultRenderHelperFuncs.ResolveUserMention(ctx, mentionedUsername)
		}
		if mentionable {
			link := "/:root/" + url.PathEscape(mentionedUsername)
			linkNode := createLink(ctx, link, mention, "" /*mention*/)
			if avatarLink != "" {
				linkNode.InsertBefore(createMentionAvatar(ctx, avatarLink), linkNode.FirstChild)
			}
			replaceContent(node, loc.Start, loc.End, linkNode)
			node = node.NextSibling.NextSibling
			start = 0
		} else {
			start = loc.End
		}
	}
}
