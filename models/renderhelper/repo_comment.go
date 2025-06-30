// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"context"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/util"
)

type RepoComment struct {
	ctx  *markup.RenderContext
	opts RepoCommentOptions

	commitChecker *commitChecker
	repoLink      string
}

func (r *RepoComment) CleanUp() {
	_ = r.commitChecker.Close()
}

func (r *RepoComment) IsCommitIDExisting(commitID string) bool {
	return r.commitChecker.IsCommitIDExisting(commitID)
}

func (r *RepoComment) ResolveLink(link, preferLinkType string) string {
	linkType, link := markup.ParseRenderedLink(link, preferLinkType)
	switch linkType {
	case markup.LinkTypeRoot:
		return r.ctx.ResolveLinkRoot(link)
	default:
		return r.ctx.ResolveLinkRelative(r.repoLink, r.opts.CurrentRefPath, link)
	}
}

var _ markup.RenderHelper = (*RepoComment)(nil)

type RepoCommentOptions struct {
	DeprecatedRepoName  string // it is only a patch for the non-standard "markup" api
	DeprecatedOwnerName string // it is only a patch for the non-standard "markup" api
	CurrentRefPath      string // eg: "branch/main" or "commit/11223344"
	FootnoteContextID   string // the extra context ID for footnotes, used to avoid conflicts with other footnotes in the same page
}

func NewRenderContextRepoComment(ctx context.Context, repo *repo_model.Repository, opts ...RepoCommentOptions) *markup.RenderContext {
	helper := &RepoComment{opts: util.OptionalArg(opts)}
	rctx := markup.NewRenderContext(ctx)
	helper.ctx = rctx
	var metas map[string]string
	if repo != nil {
		helper.repoLink = repo.Link()
		helper.commitChecker = newCommitChecker(ctx, repo)
		metas = repo.ComposeCommentMetas(ctx)
	} else {
		// repo can be nil when rendering a commit message in user's dashboard feedback whose repository has been deleted
		metas = map[string]string{}
		if helper.opts.DeprecatedOwnerName != "" {
			// this is almost dead code, only to pass the incorrect tests
			helper.repoLink = fmt.Sprintf("%s/%s", helper.opts.DeprecatedOwnerName, helper.opts.DeprecatedRepoName)
			metas["user"] = helper.opts.DeprecatedOwnerName
			metas["repo"] = helper.opts.DeprecatedRepoName
		}
		metas["markdownNewLineHardBreak"] = "true"
		metas["markupAllowShortIssuePattern"] = "true"
	}
	metas["footnoteContextId"] = helper.opts.FootnoteContextID
	rctx = rctx.WithMetas(metas).WithHelper(helper)
	return rctx
}
