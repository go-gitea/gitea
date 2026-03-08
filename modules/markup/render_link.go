// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"
)

func resolveLinkRelative(ctx context.Context, base, cur, link string, absolute bool) (finalLink string) {
	if IsFullURLString(link) {
		return link
	}
	if strings.HasPrefix(link, "/") {
		if strings.HasPrefix(link, base) && strings.Count(base, "/") >= 4 {
			// a trick to tolerate that some users were using absolute paths (the old Gitea's behavior)
			// if the link is likely "{base}/src/main" while "{base}" is something like "/owner/repo"
			finalLink = link
		} else {
			// need to resolve the link relative to "{base}"
			cur = ""
		}
	} // else: link is relative to "{base}/{cur}"

	if finalLink == "" {
		linkURL, err := url.Parse(link)
		if err != nil {
			finalLink = strings.TrimSuffix(base, "/") + path.Join("/"+cur)
		} else {
			// HINT: GOLANG-HTTP-REDIRECT-BUG: Golang security vulnerability: "http.Redirect" calls "path.Clean" and changes the meaning of a path
			linkPath := strings.ReplaceAll(linkURL.Path, "\\", "/")
			finalLink = strings.TrimSuffix(base, "/") + path.Join("/"+cur, "/"+linkPath)
			if linkURL.RawQuery != "" {
				finalLink += "?" + linkURL.RawQuery
			}
		}
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
