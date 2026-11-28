// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"gitea.dev/modules/base"
	"gitea.dev/modules/git"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/markup/common"
	"gitea.dev/modules/references"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func setCommitLinkContent(anchor *html.Node, prefix, refs, rest string) {
	anchor.Attr = append(anchor.Attr, html.Attribute{Key: "class", Val: "commit"}) // plain, so markdown strips it and commit messages keep it for CSS
	anchor.FirstChild.Data = prefix
	code := &html.Node{Type: html.ElementNode, Data: atom.Code.String()}
	code.AppendChild(&html.Node{Type: html.TextNode, Data: refs})
	anchor.AppendChild(code)
	if rest != "" {
		anchor.AppendChild(&html.Node{Type: html.TextNode, Data: rest})
	}
}

func createCommitLink(ctx *RenderContext, href, prefix, sha string) *html.Node {
	anchor := createLink(ctx, href, "", "")
	setCommitLinkContent(anchor, prefix, base.ShortSha(sha), "")
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
	if git.IsStringValidObjectID(nil, ref) {
		return base.ShortSha(ref)
	}
	return ref
}

// linkified URLs and "<url>" autolinks reach here as links whose text is their own URL
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
	var refs, rest string
	if match := globalVars().compareURLPattern.FindStringSubmatch(siteURL.RepoSubPath); match != nil {
		refs, rest = shortCompareRef(match[1])+match[2]+shortCompareRef(match[3]), match[4]
	} else if match := globalVars().commitURLPattern.FindStringSubmatch(siteURL.RepoSubPath); match != nil {
		refs, rest = base.ShortSha(match[1]), match[2]
	} else {
		return
	}
	if idx := strings.IndexAny(text.Data, "?#"); idx != -1 {
		rest += text.Data[idx:]
	}
	// quoting the link keeps its URL instead of the short text
	anchor.Attr = slices.DeleteFunc(anchor.Attr, func(attr html.Attribute) bool { return attr.Key == "data-markdown-generated-content" })
	setCommitLinkContent(anchor, otherRepoPrefix(ctx, siteURL.OwnerName, siteURL.RepoName), refs, rest)
}

// hashCurrentPatternProcessor renders SHA1 strings to corresponding links that
// are assumed to be in the same repository.
func hashCurrentPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.RenderOptions.Metas == nil || ctx.RenderOptions.Metas["user"] == "" || ctx.RenderOptions.Metas["repo"] == "" || ctx.RenderHelper == nil {
		return
	}

	text := node.Data
	if !globalVars().hashCurrentPattern.MatchString(text) {
		return
	}
	linkOrEmailRanges := append(common.GlobalVars().LinkifyRegex.FindAllStringIndex(text, -1), globalVars().emailRegex.FindAllStringIndex(text, -1)...)
	slices.SortFunc(linkOrEmailRanges, func(first, second []int) int { return cmp.Compare(first[0], second[0]) })
	start, offset := 0, 0
	for start < len(text) {
		m := globalVars().hashCurrentPattern.FindStringSubmatchIndex(text[start:])
		if m == nil {
			return
		}
		for i := range m {
			if m[i] >= 0 {
				m[i] += start
			}
		}
		end := max(m[3], m[5]) // not m[1], the consumed boundary char may lead the next match
		start = end
		for len(linkOrEmailRanges) > 0 && linkOrEmailRanges[0][1] <= m[2] {
			linkOrEmailRanges = linkOrEmailRanges[1:]
		}
		if len(linkOrEmailRanges) > 0 && linkOrEmailRanges[0][0] <= m[2] {
			continue
		}
		link := createHashLink(ctx, text, m)
		if link == nil {
			continue
		}
		replaceContent(node, m[2]-offset, end-offset, link)
		offset = end
		node = node.NextSibling.NextSibling
	}
}

func createHashLink(ctx *RenderContext, text string, m []int) *html.Node {
	fullID := ctx.RenderHelper.ResolveCommitID(text[m[2]:m[3]])
	if fullID == "" {
		return nil
	}
	repoLink := fmt.Sprintf("/:root/%s/%s/", ctx.RenderOptions.Metas["user"], ctx.RenderOptions.Metas["repo"])
	if m[4] < 0 {
		return createCommitLink(ctx, repoLink+"commit/"+fullID, "", fullID)
	}
	fullID2 := ctx.RenderHelper.ResolveCommitID(text[m[4]:m[5]])
	if fullID2 == "" {
		return nil
	}
	anchor := createLink(ctx, repoLink+"compare/"+fullID+"..."+fullID2, "", "")
	setCommitLinkContent(anchor, "", base.ShortSha(fullID)+"..."+base.ShortSha(fullID2), "")
	return anchor
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
