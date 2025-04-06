// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"strings"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

func resolveLinkRelative(ctx context.Context, base, cur, link string, absolute bool) (finalLink string) {
	if IsFullURLString(link) {
		return link
	}
	if strings.HasPrefix(link, "/") {
		if strings.HasPrefix(link, base) && strings.Count(base, "/") >= 4 {
			// a trick to tolerate that some users were using absolut paths (the old gitea's behavior)
			finalLink = link
		} else {
			finalLink = util.URLJoin(base, "./", link)
		}
	} else {
		finalLink = util.URLJoin(base, "./", cur, link)
	}
	finalLink = strings.TrimSuffix(finalLink, "/")
	if absolute {
		finalLink = httplib.MakeAbsoluteURL(ctx, finalLink)
	}
	return finalLink
}

func (ctx *RenderContext) ResolveLinkRelative(base, cur, link string) string {
	if strings.HasPrefix(link, "/:") {
		setting.PanicInDevOrTesting("invalid link %q, forgot to cut?", link)
	}
	return resolveLinkRelative(ctx, base, cur, link, ctx.RenderOptions.UseAbsoluteLink)
}

func (ctx *RenderContext) ResolveLinkRoot(link string) string {
	return ctx.ResolveLinkRelative(setting.AppSubURL+"/", "", link)
}

func ParseRenderedLink(s, preferLinkType string) (linkType, link string) {
	if strings.HasPrefix(s, "/:") {
		p := strings.IndexByte(s[1:], '/')
		if p == -1 {
			return s, ""
		}
		return s[:p+1], s[p+2:]
	}
	return preferLinkType, s
}
