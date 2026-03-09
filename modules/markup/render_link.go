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

// resolveLinkRelative tries to resolve the link relative to the "{base}/{cur}", and returns the final link.
// It only resolves the link, doesn't do any sanitization or validation, invalid links will be returned as is.
func resolveLinkRelative(ctx context.Context, base, cur, link string, absolute bool) (finalLink string) {
	linkURL, err := url.Parse(link)
	if err != nil {
		return link // invalid URL, return as is
	}
	if linkURL.Scheme != "" || linkURL.Host != "" {
		return link // absolute URL, return as is
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
		finalLink = strings.TrimSuffix(base, "/") + path.Join("/"+cur, "/"+linkURL.EscapedPath())
		finalLink = strings.TrimSuffix(finalLink, "/")
		if linkURL.RawQuery != "" {
			finalLink += "?" + linkURL.RawQuery
		}
		if linkURL.Fragment != "" {
			finalLink += "#" + linkURL.Fragment
		}
	}

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
