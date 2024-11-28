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

func (r *RepoComment) ResolveLink(link string, likeType markup.LinkType) (finalLink string) {
	switch likeType {
	case markup.LinkTypeApp:
		finalLink = r.ctx.ResolveLinkApp(link)
	default:
		finalLink = r.ctx.ResolveLinkRelative(r.repoLink, r.opts.CurrentRefPath, link)
	}
	return finalLink
}

var _ markup.RenderHelper = (*RepoComment)(nil)

type RepoCommentOptions struct {
	DeprecatedRepoName  string // it is only a patch for the non-standard "markup" api
	DeprecatedOwnerName string // it is only a patch for the non-standard "markup" api
	CurrentRefPath      string // eg: "branch/main" or "commit/11223344"
}

func NewRenderContextRepoComment(ctx context.Context, repo *repo_model.Repository, opts ...RepoCommentOptions) *markup.RenderContext {
	helper := &RepoComment{
		repoLink: repo.Link(),
		opts:     util.OptionalArg(opts),
	}
	rctx := markup.NewRenderContext(ctx)
	helper.ctx = rctx
	if repo != nil {
		helper.repoLink = repo.Link()
		helper.commitChecker = newCommitChecker(ctx, repo)
		rctx = rctx.WithMetas(repo.ComposeMetas(ctx))
	} else {
		// this is almost dead code, only to pass the incorrect tests
		helper.repoLink = fmt.Sprintf("%s/%s", helper.opts.DeprecatedOwnerName, helper.opts.DeprecatedRepoName)
		rctx = rctx.WithMetas(map[string]string{
			"user": helper.opts.DeprecatedOwnerName,
			"repo": helper.opts.DeprecatedRepoName,

			"markdownLineBreakStyle":       "comment",
			"markupAllowShortIssuePattern": "true",
		})
	}
	rctx = rctx.WithHelper(helper)
	return rctx
}
