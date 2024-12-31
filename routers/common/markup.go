// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

// RenderMarkup renders markup text for the /markup and /markdown endpoints
func RenderMarkup(ctx *context.Base, ctxRepo *context.Repository, mode, text, urlPathContext, filePath string) {
	// urlPathContext format is "/subpath/{user}/{repo}/src/{branch, commit, tag}/{identifier/path}/{file/dir}"
	// filePath is the path of the file to render if the end user is trying to preview a repo file (mode == "file")
	// filePath will be used as RenderContext.RelativePath

	// for example, when previewing file "/gitea/owner/repo/src/branch/features/feat-123/doc/CHANGE.md", then filePath is "doc/CHANGE.md"
	// and the urlPathContext is "/gitea/owner/repo/src/branch/features/feat-123/doc"

	if mode == "" || mode == "markdown" {
		// raw markdown doesn't need any special handling
		baseLink := urlPathContext
		if baseLink == "" {
			baseLink = fmt.Sprintf("%s%s", httplib.GuessCurrentHostURL(ctx), urlPathContext)
		}
		rctx := renderhelper.NewRenderContextSimpleDocument(ctx, baseLink).WithUseAbsoluteLink(true).
			WithMarkupType(markdown.MarkupName)
		if err := markdown.RenderRaw(rctx, strings.NewReader(text), ctx.Resp); err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Ideally, this handler should be called with RepoAssigment and get the related repo from context "/owner/repo/markup"
	// then render could use the repo to do various things (the permission check has passed)
	//
	// However, this handler is also exposed as "/markup" without any repo context,
	// then since there is no permission check, so we can't use the repo from "context" parameter,
	// in this case, only the "path" information could be used which doesn't cause security problems.
	var repoModel *repo.Repository
	if ctxRepo != nil {
		repoModel = ctxRepo.Repository
	}
	var repoOwnerName, repoName, refPath, treePath string
	repoLinkPath := strings.TrimPrefix(urlPathContext, setting.AppSubURL+"/")
	fields := strings.SplitN(repoLinkPath, "/", 5)
	if len(fields) == 5 && fields[2] == "src" && (fields[3] == "branch" || fields[3] == "commit" || fields[3] == "tag") {
		// absolute base prefix is something like "https://host/subpath/{user}/{repo}"
		repoOwnerName, repoName = fields[0], fields[1]
		treePath = path.Dir(filePath)                       // it is "doc" if filePath is "doc/CHANGE.md"
		refPath = strings.Join(fields[3:], "/")             // it is "branch/features/feat-12/doc"
		refPath = strings.TrimSuffix(refPath, "/"+treePath) // now we get the correct branch path: "branch/features/feat-12"
	} else if fields = strings.SplitN(repoLinkPath, "/", 3); len(fields) == 2 {
		repoOwnerName, repoName = fields[0], fields[1]
	}

	var rctx *markup.RenderContext
	switch mode {
	case "gfm": // legacy mode
		rctx = renderhelper.NewRenderContextRepoFile(ctx, repoModel, renderhelper.RepoFileOptions{
			DeprecatedOwnerName: repoOwnerName, DeprecatedRepoName: repoName,
			CurrentRefPath: refPath, CurrentTreePath: treePath,
		})
		rctx = rctx.WithMarkupType(markdown.MarkupName)
	case "comment":
		rctx = renderhelper.NewRenderContextRepoComment(ctx, repoModel, renderhelper.RepoCommentOptions{DeprecatedOwnerName: repoOwnerName, DeprecatedRepoName: repoName})
		rctx = rctx.WithMarkupType(markdown.MarkupName)
	case "wiki":
		rctx = renderhelper.NewRenderContextRepoWiki(ctx, repoModel, renderhelper.RepoWikiOptions{DeprecatedOwnerName: repoOwnerName, DeprecatedRepoName: repoName})
		rctx = rctx.WithMarkupType(markdown.MarkupName)
	case "file":
		rctx = renderhelper.NewRenderContextRepoFile(ctx, repoModel, renderhelper.RepoFileOptions{
			DeprecatedOwnerName: repoOwnerName, DeprecatedRepoName: repoName,
			CurrentRefPath: refPath, CurrentTreePath: treePath,
		})
		rctx = rctx.WithMarkupType("").WithRelativePath(filePath) // render the repo file content by its extension
	default:
		ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Unknown mode: %s", mode))
		return
	}
	rctx = rctx.WithUseAbsoluteLink(true)
	if err := markup.Render(rctx, strings.NewReader(text), ctx.Resp); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusUnprocessableEntity, err.Error())
		} else {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	}
}
