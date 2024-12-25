// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/regexplru"
	"code.gitea.io/gitea/modules/templates/vars"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type RenderIssueIconTitleOptions struct {
	OwnerName  string
	RepoName   string
	LinkHref   string
	IssueIndex int64
}

func fullIssuePatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.RenderOptions.Metas == nil {
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
		if !httplib.IsCurrentGiteaSiteURL(ctx, link) {
			return
		}
		text := "#" + node.Data[m[2]:m[3]]
		// if m[4] and m[5] is not -1, then link is to a comment
		// indicate that in the text by appending (comment)
		if m[4] != -1 && m[5] != -1 {
			if locale, ok := ctx.Value(translation.ContextKey).(translation.Locale); ok {
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

		if matchOrg == ctx.RenderOptions.Metas["user"] && matchRepo == ctx.RenderOptions.Metas["repo"] {
			replaceContent(node, m[0], m[1], createLink(ctx, link, text, "ref-issue"))
		} else {
			text = matchOrg + "/" + matchRepo + text
			replaceContent(node, m[0], m[1], createLink(ctx, link, text, "ref-issue"))
		}
		node = node.NextSibling.NextSibling
	}
}

func createIssueLinkContentWithSummary(ctx *RenderContext, linkHref string, ref *references.RenderizableReference) *html.Node {
	if DefaultRenderHelperFuncs.RenderRepoIssueIconTitle == nil {
		return nil
	}
	issueIndex, _ := strconv.ParseInt(ref.Issue, 10, 64)
	h, err := DefaultRenderHelperFuncs.RenderRepoIssueIconTitle(ctx, RenderIssueIconTitleOptions{
		OwnerName:  ref.Owner,
		RepoName:   ref.Name,
		LinkHref:   linkHref,
		IssueIndex: issueIndex,
	})
	if err != nil {
		log.Error("RenderRepoIssueIconTitle failed: %v", err)
		return nil
	}
	if h == "" {
		return nil
	}
	return &html.Node{Type: html.RawNode, Data: string(ctx.RenderInternal.ProtectSafeAttrs(h))}
}

func issueIndexPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.RenderOptions.Metas == nil {
		return
	}

	// crossLinkOnly: do not parse "#123", only parse "owner/repo#123"
	// if there is no repo in the context, then the "#123" format can't be parsed
	// old logic: crossLinkOnly := ctx.RenderOptions.Metas["mode"] == "document" && !ctx.IsWiki
	crossLinkOnly := ctx.RenderOptions.Metas["markupAllowShortIssuePattern"] != "true"

	var ref *references.RenderizableReference

	next := node.NextSibling
	for node != nil && node != next {
		_, hasExtTrackFormat := ctx.RenderOptions.Metas["format"]

		// Repos with external issue trackers might still need to reference local PRs
		// We need to concern with the first one that shows up in the text, whichever it is
		isNumericStyle := ctx.RenderOptions.Metas["style"] == "" || ctx.RenderOptions.Metas["style"] == IssueNameStyleNumeric
		refNumeric := references.FindRenderizableReferenceNumeric(node.Data, hasExtTrackFormat && !isNumericStyle, crossLinkOnly)

		switch ctx.RenderOptions.Metas["style"] {
		case "", IssueNameStyleNumeric:
			ref = refNumeric
		case IssueNameStyleAlphanumeric:
			ref = references.FindRenderizableReferenceAlphanumeric(node.Data)
		case IssueNameStyleRegexp:
			pattern, err := regexplru.GetCompiled(ctx.RenderOptions.Metas["regexp"])
			if err != nil {
				return
			}
			ref = references.FindRenderizableReferenceRegexp(node.Data, pattern)
		}

		// Repos with external issue trackers might still need to reference local PRs
		// We need to concern with the first one that shows up in the text, whichever it is
		if hasExtTrackFormat && !isNumericStyle && refNumeric != nil {
			// If numeric (PR) was found, and it was BEFORE the non-numeric pattern, use that
			// Allow a free-pass when non-numeric pattern wasn't found.
			if ref == nil || refNumeric.RefLocation.Start < ref.RefLocation.Start {
				ref = refNumeric
			}
		}

		if ref == nil {
			return
		}

		var link *html.Node
		refText := node.Data[ref.RefLocation.Start:ref.RefLocation.End]
		if hasExtTrackFormat && !ref.IsPull {
			ctx.RenderOptions.Metas["index"] = ref.Issue

			res, err := vars.Expand(ctx.RenderOptions.Metas["format"], ctx.RenderOptions.Metas)
			if err != nil {
				// here we could just log the error and continue the rendering
				log.Error("unable to expand template vars for ref %s, err: %v", ref.Issue, err)
			}

			link = createLink(ctx, res, refText, "ref-issue ref-external-issue")
		} else {
			// Path determines the type of link that will be rendered. It's unknown at this point whether
			// the linked item is actually a PR or an issue. Luckily it's of no real consequence because
			// Gitea will redirect on click as appropriate.
			issueOwner := util.Iif(ref.Owner == "", ctx.RenderOptions.Metas["user"], ref.Owner)
			issueRepo := util.Iif(ref.Owner == "", ctx.RenderOptions.Metas["repo"], ref.Name)
			issuePath := util.Iif(ref.IsPull, "pulls", "issues")
			linkHref := ctx.RenderHelper.ResolveLink(util.URLJoin(issueOwner, issueRepo, issuePath, ref.Issue), LinkTypeApp)

			// at the moment, only render the issue index in a full line (or simple line) as icon+title
			// otherwise it would be too noisy for "take #1 as an example" in a sentence
			if node.Parent.DataAtom == atom.Li && ref.RefLocation.Start < 20 && ref.RefLocation.End == len(node.Data) {
				link = createIssueLinkContentWithSummary(ctx, linkHref, ref)
			}
			if link == nil {
				link = createLink(ctx, linkHref, refText, "ref-issue")
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
			keyword = createKeyword(ctx, node.Data[ref.ActionLocation.Start:ref.ActionLocation.End])
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
