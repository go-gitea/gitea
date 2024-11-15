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

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

// RenderMarkup renders markup text for the /markup and /markdown endpoints
func RenderMarkup(ctx *context.Base, repo *context.Repository, mode, text, urlPathContext, filePath string) {
	// urlPathContext format is "/subpath/{user}/{repo}/src/{branch, commit, tag}/{identifier/path}/{file/dir}"
	// filePath is the path of the file to render if the end user is trying to preview a repo file (mode == "file")
	// filePath will be used as RenderContext.RelativePath

	// for example, when previewing file "/gitea/owner/repo/src/branch/features/feat-123/doc/CHANGE.md", then filePath is "doc/CHANGE.md"
	// and the urlPathContext is "/gitea/owner/repo/src/branch/features/feat-123/doc"

	renderCtx := &markup.RenderContext{
		Ctx:        ctx,
		Links:      markup.Links{AbsolutePrefix: true},
		MarkupType: markdown.MarkupName,
	}
	if urlPathContext != "" {
		renderCtx.Links.Base = fmt.Sprintf("%s%s", httplib.GuessCurrentHostURL(ctx), urlPathContext)
	}

	if mode == "" || mode == "markdown" {
		// raw markdown doesn't need any special handling
		if err := markdown.RenderRaw(renderCtx, strings.NewReader(text), ctx.Resp); err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	}
	switch mode {
	case "gfm": // legacy mode, do nothing
	case "comment":
		renderCtx.ContentMode = markup.RenderContentAsComment
	case "wiki":
		renderCtx.ContentMode = markup.RenderContentAsWiki
	case "file":
		// render the repo file content by its extension
		renderCtx.MarkupType = ""
		renderCtx.RelativePath = filePath
		renderCtx.InStandalonePage = true
	default:
		ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Unknown mode: %s", mode))
		return
	}

	fields := strings.SplitN(strings.TrimPrefix(urlPathContext, setting.AppSubURL+"/"), "/", 5)
	if len(fields) == 5 && fields[2] == "src" && (fields[3] == "branch" || fields[3] == "commit" || fields[3] == "tag") {
		// absolute base prefix is something like "https://host/subpath/{user}/{repo}"
		absoluteBasePrefix := fmt.Sprintf("%s%s/%s", httplib.GuessCurrentAppURL(ctx), fields[0], fields[1])

		fileDir := path.Dir(filePath)                      // it is "doc" if filePath is "doc/CHANGE.md"
		refPath := strings.Join(fields[3:], "/")           // it is "branch/features/feat-12/doc"
		refPath = strings.TrimSuffix(refPath, "/"+fileDir) // now we get the correct branch path: "branch/features/feat-12"

		renderCtx.Links = markup.Links{AbsolutePrefix: true, Base: absoluteBasePrefix, BranchPath: refPath, TreePath: fileDir}
	}

	if repo != nil && repo.Repository != nil {
		renderCtx.Repo = repo.Repository
		if renderCtx.ContentMode == markup.RenderContentAsComment {
			renderCtx.Metas = repo.Repository.ComposeMetas(ctx)
		} else {
			renderCtx.Metas = repo.Repository.ComposeDocumentMetas(ctx)
		}
	}
	if err := markup.Render(renderCtx, strings.NewReader(text), ctx.Resp); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusUnprocessableEntity, err.Error())
		} else {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	}
}
