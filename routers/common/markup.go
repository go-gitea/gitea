// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

// RenderMarkup renders markup text for the /markup and /markdown endpoints
func RenderMarkup(ctx *context.Base, repo *context.Repository, mode, text, urlPathContext, filePath string, wiki bool) {
	// urlPathContext format is "/subpath/{user}/{repo}/src/{branch, commit, tag}/{identifier/path}/{file/dir}"
	// filePath is the path of the file to render if the end user is trying to preview a repo file (mode == "file")
	// filePath will be used as RenderContext.RelativePath

	// for example, when previewing file "/gitea/owner/repo/src/branch/features/feat-123/doc/CHANGE.md", then filePath is "doc/CHANGE.md"
	// and the urlPathContext is "/gitea/owner/repo/src/branch/features/feat-123/doc"

	var markupType, relativePath string

	links := markup.Links{AbsolutePrefix: true}
	if urlPathContext != "" {
		links.Base = fmt.Sprintf("%s%s", httplib.GuessCurrentHostURL(ctx), urlPathContext)
	}

	switch mode {
	case "markdown":
		// Raw markdown
		if err := markdown.RenderRaw(&markup.RenderContext{
			Ctx:   ctx,
			Links: links,
		}, strings.NewReader(text), ctx.Resp); err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	case "comment":
		// Issue & comment content
		markupType = markdown.MarkupName
	case "gfm":
		// GitHub Flavored Markdown
		markupType = markdown.MarkupName
	case "file":
		markupType = "" // render the repo file content by its extension
		relativePath = filePath
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

		links = markup.Links{AbsolutePrefix: true, Base: absoluteBasePrefix, BranchPath: refPath, TreePath: fileDir}
	}

	meta := map[string]string{}
	var repoCtx *repo_model.Repository
	if repo != nil && repo.Repository != nil {
		repoCtx = repo.Repository
		if mode == "comment" {
			meta = repo.Repository.ComposeMetas(ctx)
		} else {
			meta = repo.Repository.ComposeDocumentMetas(ctx)
		}
	}
	if mode != "comment" {
		meta["mode"] = "document"
	}

	if err := markup.Render(&markup.RenderContext{
		Ctx:          ctx,
		Repo:         repoCtx,
		Links:        links,
		Metas:        meta,
		IsWiki:       wiki,
		Type:         markupType,
		RelativePath: relativePath,
	}, strings.NewReader(text), ctx.Resp); err != nil {
		if markup.IsErrUnsupportedRenderExtension(err) {
			ctx.Error(http.StatusUnprocessableEntity, err.Error())
		} else {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	}
}
