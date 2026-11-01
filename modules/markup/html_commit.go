// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"fmt"
	"slices"
	"strings"

	"gitea.dev/modules/httplib"
	"gitea.dev/modules/references"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func setCommitLinkContent(anchor *html.Node, prefix, refs, rest string) {
	anchor.FirstChild.Data = prefix
	code := &html.Node{Type: html.ElementNode, Data: atom.Code.String()}
	code.AppendChild(&html.Node{Type: html.TextNode, Data: refs})
	anchor.AppendChild(code)
	if rest != "" {
		anchor.AppendChild(&html.Node{Type: html.TextNode, Data: rest})
	}
}

func createCommitLink(ctx *RenderContext, href, prefix, sha string) *html.Node {
	anchor := createLink(ctx, href, "", "commit")
	setCommitLinkContent(anchor, prefix, sha[:7], "")
	return anchor
}

func otherRepoPrefix(ctx *RenderContext, owner, repo string) string {
	if strings.EqualFold(owner, ctx.RenderOptions.Metas["user"]) && strings.EqualFold(repo, ctx.RenderOptions.Metas["repo"]) {
		return ""
	}
	return owner + "/" + repo + "@"
}

// compare refs can also be branch or tag names, so only full SHAs are shortened
func shortCompareRef(ref string) string {
	if (len(ref) == 40 || len(ref) == 64) && strings.Trim(ref, "0123456789abcdef") == "" {
		return ref[:7]
	}
	return ref
}

// shortenCommitLink shortens links whose text is their own commit or compare URL, like linkified URLs and "<url>" autolinks
func shortenCommitLink(ctx *RenderContext, anchor *html.Node) {
	text := anchor.FirstChild
	if ctx.RenderOptions.Metas == nil || text == nil || text != anchor.LastChild || text.Type != html.TextNode || !strings.Contains(text.Data, "://") {
		return
	}
	if !slices.ContainsFunc(anchor.Attr, func(attr html.Attribute) bool { return attr.Key == "href" && attr.Val == text.Data }) {
		return
	}
	siteURL := httplib.ParseGiteaSiteURL(ctx, text.Data)
	if siteURL == nil {
		return
	}
	var refs, rest, class string
	if match := globalVars().compareURLPattern.FindStringSubmatch(siteURL.RepoSubPath); match != nil {
		refs, rest, class = shortCompareRef(match[1])+match[2]+shortCompareRef(match[3]), match[4], "compare"
	} else if match := globalVars().commitURLPattern.FindStringSubmatch(siteURL.RepoSubPath); match != nil {
		refs, rest, class = match[1][:7], match[2], "commit"
	} else {
		return
	}
	if idx := strings.IndexAny(text.Data, "?#"); idx != -1 {
		rest += text.Data[idx:]
	}
	// quoting the link keeps its URL instead of the short text
	anchor.Attr = slices.DeleteFunc(anchor.Attr, func(attr html.Attribute) bool { return attr.Key == "data-markdown-generated-content" })
	anchor.Attr = append(anchor.Attr, ctx.RenderInternal.NodeSafeAttr("class", class))
	setCommitLinkContent(anchor, otherRepoPrefix(ctx, siteURL.OwnerName, siteURL.RepoName), refs, rest)
}

// hashCurrentPatternProcessor renders SHA1 strings to corresponding links that
// are assumed to be in the same repository.
func hashCurrentPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.RenderOptions.Metas == nil || ctx.RenderOptions.Metas["user"] == "" || ctx.RenderOptions.Metas["repo"] == "" || ctx.RenderHelper == nil {
		return
	}

	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := globalVars().hashCurrentPattern.FindStringSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[2] += start
		m[3] += start

		hash := node.Data[m[2]:m[3]]
		// The regex does not lie, it matches the hash pattern.
		// However, a regex cannot know if a hash actually exists or not.
		// We could assume that a SHA1 hash should probably contain alphas AND numerics
		// but that is not always the case.
		// Although unlikely, deadbeef and 1234567 are valid short forms of SHA1 hash
		// as used by git and github for linking and thus we have to do similar.
		// Because of this, we check to make sure that a matched hash is actually
		// a commit in the repository before making it a link.
		if !ctx.RenderHelper.IsCommitIDExisting(hash) {
			start = m[3]
			continue
		}

		link := fmt.Sprintf("/:root/%s/%s/commit/%s", ctx.RenderOptions.Metas["user"], ctx.RenderOptions.Metas["repo"], hash)
		replaceContent(node, m[2], m[3], createCommitLink(ctx, link, "", hash))
		start = 0
		node = node.NextSibling.NextSibling
	}
}

func commitCrossReferencePatternProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling

	for node != nil && node != next {
		found, ref := references.FindRenderizableCommitCrossReference(node.Data)
		if !found {
			return
		}

		linkHref := fmt.Sprintf("/:root/%s/%s/commit/%s", ref.Owner, ref.Name, ref.CommitSha)
		link := createCommitLink(ctx, linkHref, otherRepoPrefix(ctx, ref.Owner, ref.Name), ref.CommitSha)

		replaceContent(node, ref.RefLocation.Start, ref.RefLocation.End, link)
		node = node.NextSibling.NextSibling
	}
}
