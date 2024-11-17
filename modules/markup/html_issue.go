// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/regexplru"
	"code.gitea.io/gitea/modules/templates/vars"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/net/html"
)

func fullIssuePatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}
	next := node.NextSibling
	for node != nil && node != next {
		m := globalVars().issueFullPattern.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		mDiffView := globalVars().filesChangedFullPattern.FindStringSubmatchIndex(node.Data)
		// leave it as it is if the link is from "Files Changed" tab in PR Diff View https://domain/org/repo/pulls/27/files
		if mDiffView != nil {
			return
		}

		link := node.Data[m[0]:m[1]]
		if !httplib.IsCurrentGiteaSiteURL(ctx.Ctx, link) {
			return
		}
		text := "#" + node.Data[m[2]:m[3]]
		// if m[4] and m[5] is not -1, then link is to a comment
		// indicate that in the text by appending (comment)
		if m[4] != -1 && m[5] != -1 {
			if locale, ok := ctx.Ctx.Value(translation.ContextKey).(translation.Locale); ok {
				text += " " + locale.TrString("repo.from_comment")
			} else {
				text += " (comment)"
			}
		}

		// extract repo and org name from matched link like
		// http://localhost:3000/gituser/myrepo/issues/1
		linkParts := strings.Split(link, "/")
		matchOrg := linkParts[len(linkParts)-4]
		matchRepo := linkParts[len(linkParts)-3]

		if matchOrg == ctx.Metas["user"] && matchRepo == ctx.Metas["repo"] {
			replaceContent(node, m[0], m[1], createLink(link, text, "ref-issue"))
		} else {
			text = matchOrg + "/" + matchRepo + text
			replaceContent(node, m[0], m[1], createLink(link, text, "ref-issue"))
		}
		node = node.NextSibling.NextSibling
	}
}

func issueIndexPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}

	// crossLinkOnly: do not parse "#123", only parse "owner/repo#123"
	// if there is no repo in the context, then the "#123" format can't be parsed
	// old logic: crossLinkOnly := ctx.Metas["mode"] == "document" && !ctx.IsWiki
	crossLinkOnly := ctx.Metas["markupAllowShortIssuePattern"] != "true"

	var (
		found bool
		ref   *references.RenderizableReference
	)

	next := node.NextSibling

	for node != nil && node != next {
		_, hasExtTrackFormat := ctx.Metas["format"]

		// Repos with external issue trackers might still need to reference local PRs
		// We need to concern with the first one that shows up in the text, whichever it is
		isNumericStyle := ctx.Metas["style"] == "" || ctx.Metas["style"] == IssueNameStyleNumeric
		foundNumeric, refNumeric := references.FindRenderizableReferenceNumeric(node.Data, hasExtTrackFormat && !isNumericStyle, crossLinkOnly)

		switch ctx.Metas["style"] {
		case "", IssueNameStyleNumeric:
			found, ref = foundNumeric, refNumeric
		case IssueNameStyleAlphanumeric:
			found, ref = references.FindRenderizableReferenceAlphanumeric(node.Data)
		case IssueNameStyleRegexp:
			pattern, err := regexplru.GetCompiled(ctx.Metas["regexp"])
			if err != nil {
				return
			}
			found, ref = references.FindRenderizableReferenceRegexp(node.Data, pattern)
		}

		// Repos with external issue trackers might still need to reference local PRs
		// We need to concern with the first one that shows up in the text, whichever it is
		if hasExtTrackFormat && !isNumericStyle && refNumeric != nil {
			// If numeric (PR) was found, and it was BEFORE the non-numeric pattern, use that
			// Allow a free-pass when non-numeric pattern wasn't found.
			if found && (ref == nil || refNumeric.RefLocation.Start < ref.RefLocation.Start) {
				found = foundNumeric
				ref = refNumeric
			}
		}
		if !found {
			return
		}

		var link *html.Node
		reftext := node.Data[ref.RefLocation.Start:ref.RefLocation.End]
		if hasExtTrackFormat && !ref.IsPull {
			ctx.Metas["index"] = ref.Issue

			res, err := vars.Expand(ctx.Metas["format"], ctx.Metas)
			if err != nil {
				// here we could just log the error and continue the rendering
				log.Error("unable to expand template vars for ref %s, err: %v", ref.Issue, err)
			}

			link = createLink(res, reftext, "ref-issue ref-external-issue")
		} else {
			// Path determines the type of link that will be rendered. It's unknown at this point whether
			// the linked item is actually a PR or an issue. Luckily it's of no real consequence because
			// Gitea will redirect on click as appropriate.
			issuePath := util.Iif(ref.IsPull, "pulls", "issues")
			if ref.Owner == "" {
				link = createLink(util.URLJoin(ctx.Links.Prefix(), ctx.Metas["user"], ctx.Metas["repo"], issuePath, ref.Issue), reftext, "ref-issue")
			} else {
				link = createLink(util.URLJoin(ctx.Links.Prefix(), ref.Owner, ref.Name, issuePath, ref.Issue), reftext, "ref-issue")
			}
		}

		if ref.Action == references.XRefActionNone {
			replaceContent(node, ref.RefLocation.Start, ref.RefLocation.End, link)
			node = node.NextSibling.NextSibling
			continue
		}

		// Decorate action keywords if actionable
		var keyword *html.Node
		if references.IsXrefActionable(ref, hasExtTrackFormat) {
			keyword = createKeyword(node.Data[ref.ActionLocation.Start:ref.ActionLocation.End])
		} else {
			keyword = &html.Node{
				Type: html.TextNode,
				Data: node.Data[ref.ActionLocation.Start:ref.ActionLocation.End],
			}
		}
		spaces := &html.Node{
			Type: html.TextNode,
			Data: node.Data[ref.ActionLocation.End:ref.RefLocation.Start],
		}
		replaceContentList(node, ref.ActionLocation.Start, ref.RefLocation.End, []*html.Node{keyword, spaces, link})
		node = node.NextSibling.NextSibling.NextSibling.NextSibling
	}
}

func commitCrossReferencePatternProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling

	for node != nil && node != next {
		found, ref := references.FindRenderizableCommitCrossReference(node.Data)
		if !found {
			return
		}

		reftext := ref.Owner + "/" + ref.Name + "@" + base.ShortSha(ref.CommitSha)
		link := createLink(util.URLJoin(ctx.Links.Prefix(), ref.Owner, ref.Name, "commit", ref.CommitSha), reftext, "commit")

		replaceContent(node, ref.RefLocation.Start, ref.RefLocation.End, link)
		node = node.NextSibling.NextSibling
	}
}
