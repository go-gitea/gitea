// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"context"
	"fmt"
	"path"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/util"
)

type RepoWiki struct {
	ctx  *markup.RenderContext
	opts RepoWikiOptions

	commitChecker *commitChecker
	repoLink      string
}

func (r *RepoWiki) CleanUp() {
	_ = r.commitChecker.Close()
}

func (r *RepoWiki) IsCommitIDExisting(commitID string) bool {
	return r.commitChecker.IsCommitIDExisting(commitID)
}

func (r *RepoWiki) ResolveLink(link, preferLinkType string) (finalLink string) {
	linkType, link := markup.ParseRenderedLink(link, preferLinkType)
	switch linkType {
	case markup.LinkTypeRoot:
		finalLink = r.ctx.ResolveLinkRoot(link)
	case markup.LinkTypeMedia, markup.LinkTypeRaw:
		finalLink = r.ctx.ResolveLinkRelative(path.Join(r.repoLink, "wiki/raw", r.opts.currentRefPath), r.opts.currentTreePath, link)
	default:
		finalLink = r.ctx.ResolveLinkRelative(path.Join(r.repoLink, "wiki", r.opts.currentRefPath), r.opts.currentTreePath, link)
	}
	return finalLink
}

var _ markup.RenderHelper = (*RepoWiki)(nil)

type RepoWikiOptions struct {
	DeprecatedRepoName  string // it is only a patch for the non-standard "markup" api
	DeprecatedOwnerName string // it is only a patch for the non-standard "markup" api

	// these options are not used at the moment because Wiki doesn't support sub-path, nor branch
	currentRefPath  string // eg: "branch/main"
	currentTreePath string // eg: "path/to/file" in the repo
}

func NewRenderContextRepoWiki(ctx context.Context, repo *repo_model.Repository, opts ...RepoWikiOptions) *markup.RenderContext {
	helper := &RepoWiki{opts: util.OptionalArg(opts)}
	rctx := markup.NewRenderContext(ctx).WithMarkupType(markdown.MarkupName)
	if repo != nil {
		helper.repoLink = repo.Link()
		helper.commitChecker = newCommitChecker(ctx, repo)
		rctx = rctx.WithMetas(repo.ComposeWikiMetas(ctx))
	} else {
		// this is almost dead code, only to pass the incorrect tests
		helper.repoLink = fmt.Sprintf("%s/%s", helper.opts.DeprecatedOwnerName, helper.opts.DeprecatedRepoName)
		rctx = rctx.WithMetas(map[string]string{
			"user": helper.opts.DeprecatedOwnerName,
			"repo": helper.opts.DeprecatedRepoName,

			"markupAllowShortIssuePattern": "true",
		})
	}
	rctx = rctx.WithHelper(helper)
	helper.ctx = rctx
	return rctx
}
