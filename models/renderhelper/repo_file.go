// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderhelper

import (
	"context"
	"fmt"
	"path"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/util"
)

type RepoFile struct {
	ctx  *markup.RenderContext
	opts RepoFileOptions

	commitChecker *commitChecker
	repoLink      string
}

func (r *RepoFile) CleanUp() {
	_ = r.commitChecker.Close()
}

func (r *RepoFile) IsCommitIDExisting(commitID string) bool {
	return r.commitChecker.IsCommitIDExisting(commitID)
}

func (r *RepoFile) ResolveLink(link, preferLinkType string) (finalLink string) {
	linkType, link := markup.ParseRenderedLink(link, preferLinkType)
	switch linkType {
	case markup.LinkTypeRoot:
		finalLink = r.ctx.ResolveLinkRoot(link)
	case markup.LinkTypeRaw:
		finalLink = r.ctx.ResolveLinkRelative(path.Join(r.repoLink, "raw", r.opts.CurrentRefPath), r.opts.CurrentTreePath, link)
	case markup.LinkTypeMedia:
		finalLink = r.ctx.ResolveLinkRelative(path.Join(r.repoLink, "media", r.opts.CurrentRefPath), r.opts.CurrentTreePath, link)
	default:
		finalLink = r.ctx.ResolveLinkRelative(path.Join(r.repoLink, "src", r.opts.CurrentRefPath), r.opts.CurrentTreePath, link)
	}
	return finalLink
}

var _ markup.RenderHelper = (*RepoFile)(nil)

type RepoFileOptions struct {
	DeprecatedRepoName  string // it is only a patch for the non-standard "markup" api
	DeprecatedOwnerName string // it is only a patch for the non-standard "markup" api

	CurrentRefPath  string // eg: "branch/main"
	CurrentTreePath string // eg: "path/to/file" in the repo
}

func NewRenderContextRepoFile(ctx context.Context, repo *repo_model.Repository, opts ...RepoFileOptions) *markup.RenderContext {
	helper := &RepoFile{opts: util.OptionalArg(opts)}
	rctx := markup.NewRenderContext(ctx)
	helper.ctx = rctx
	if repo != nil {
		helper.repoLink = repo.Link()
		helper.commitChecker = newCommitChecker(ctx, repo)
		rctx = rctx.WithMetas(repo.ComposeRepoFileMetas(ctx))
	} else {
		// this is almost dead code, only to pass the incorrect tests
		helper.repoLink = fmt.Sprintf("%s/%s", helper.opts.DeprecatedOwnerName, helper.opts.DeprecatedRepoName)
		rctx = rctx.WithMetas(map[string]string{
			"user": helper.opts.DeprecatedOwnerName,
			"repo": helper.opts.DeprecatedRepoName,
		})
	}
	rctx = rctx.WithHelper(helper)
	return rctx
}
