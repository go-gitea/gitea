// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"path"

	"code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/services/context"
)

// RenderFile renders a file by repos path
func RenderFile(ctx *context.Context) {
	var blob *git.Blob
	var err error
	if ctx.Repo.TreePath != "" {
		blob, err = ctx.Repo.Commit.GetBlobByPath(ctx.Repo.TreePath)
	} else {
		blob, err = ctx.Repo.GitRepo.GetBlob(ctx.PathParam("sha"))
	}
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetBlobByPath", err)
		}
		return
	}

	dataRc, err := blob.DataAsync()
	if err != nil {
		ctx.ServerError("DataAsync", err)
		return
	}
	defer dataRc.Close()

	if markupType := markup.DetectMarkupTypeByFileName(blob.Name()); markupType == "" {
		http.Error(ctx.Resp, "Unsupported file type render", http.StatusBadRequest)
		return
	}

	rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
		CurrentRefPath:  ctx.Repo.RefTypeNameSubURL(),
		CurrentTreePath: path.Dir(ctx.Repo.TreePath),
	}).WithRelativePath(ctx.Repo.TreePath).WithInStandalonePage(true)

	renderer, err := markup.FindRendererByContext(rctx)
	if err != nil {
		http.Error(ctx.Resp, "Unable to find renderer", http.StatusBadRequest)
		return
	}

	extRenderer, ok := renderer.(markup.ExternalRenderer)
	if !ok {
		http.Error(ctx.Resp, "Unable to get external renderer", http.StatusBadRequest)
		return
	}

	// To render PDF in iframe, the sandbox must NOT be used (iframe & CSP header).
	// Chrome blocks the PDF rendering when sandboxed, even if all "allow-*" are set.
	// HINT: PDF-RENDER-SANDBOX: PDF won't render in sandboxed context
	extRendererOpts := extRenderer.GetExternalRendererOptions()
	if extRendererOpts.ContentSandbox != "" {
		ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'; sandbox "+extRendererOpts.ContentSandbox)
	} else {
		ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'")
	}

	err = markup.RenderWithRenderer(rctx, renderer, dataRc, ctx.Resp)
	if err != nil {
		log.Error("Failed to render file %q: %v", ctx.Repo.TreePath, err)
		http.Error(ctx.Resp, "Failed to render file", http.StatusInternalServerError)
		return
	}
}
