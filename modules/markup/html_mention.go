// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"

	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/util"

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
		teams, ok := ctx.RenderOptions.Metas["teams"]
		// FIXME: util.URLJoin may not be necessary here:
		// - setting.AppURL is defined to have a terminal '/' so unless mention[1:]
		// is an AppSubURL link we can probably fallback to concatenation.
		// team mention should follow @orgName/teamName style
		if ok && strings.Contains(mention, "/") {
			mentionOrgAndTeam := strings.Split(mention, "/")
			if mentionOrgAndTeam[0][1:] == ctx.RenderOptions.Metas["org"] && strings.Contains(teams, ","+strings.ToLower(mentionOrgAndTeam[1])+",") {
				link := ctx.RenderHelper.ResolveLink(util.URLJoin("org", ctx.RenderOptions.Metas["org"], "teams", mentionOrgAndTeam[1]), LinkTypeApp)
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
			link := ctx.RenderHelper.ResolveLink(mentionedUsername, LinkTypeApp)
			replaceContent(node, loc.Start, loc.End, createLink(ctx, link, mention, "" /*mention*/))
			node = node.NextSibling.NextSibling
			start = 0
		} else {
			start = loc.End
		}
	}
}
