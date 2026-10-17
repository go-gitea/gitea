// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"fmt"
	"net/url"
	"strings"

	"gitea.dev/modules/references"
	"gitea.dev/modules/util"

	"golang.org/x/net/html"
)

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

		if DefaultRenderHelperFuncs != nil && DefaultRenderHelperFuncs.IsUsernameMentionable(ctx, mentionedUsername) {
			link := "/:root/" + url.PathEscape(mentionedUsername)
			replaceContent(node, loc.Start, loc.End, createLink(ctx, link, mention, "" /*mention*/))
			node = node.NextSibling.NextSibling
			start = 0
		} else {
			start = loc.End
		}
	}
}
