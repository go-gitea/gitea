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

	blobReader, err := blob.DataAsync()
	if err != nil {
		ctx.ServerError("DataAsync", err)
		return
	}
	defer blobReader.Close()

	rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
		CurrentRefPath:  ctx.Repo.RefTypeNameSubURL(),
		CurrentTreePath: path.Dir(ctx.Repo.TreePath),
	}).WithRelativePath(ctx.Repo.TreePath).WithStandalonePage(markup.StandalonePageOptions{
		CurrentWebTheme:   ctx.TemplateContext.CurrentWebTheme(),
		RenderQueryString: ctx.Req.URL.RawQuery,
	})
	renderer, rendererInput, err := rctx.DetectMarkupRendererByReader(blobReader)
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
	switch {
	case extRendererOpts.SrcMethod == "src":
		// The iframe is loaded via "src", so the response must NOT carry the CSP "sandbox"
		// directive (Firefox refuses same-origin src loading of a sandboxed response with
		// "Unsafe attempt to load URL ..."). Isolation is enforced by the iframe element's
		// sandbox attribute instead. The script-src here is permissive enough to allow
		// WebAssembly (asciinema-player), while the main site CSP stays untouched.
		ctx.Resp.Header().Add("Content-Security-Policy",
			"frame-src 'self'; script-src * 'unsafe-inline' 'wasm-unsafe-eval'; style-src * 'unsafe-inline'; default-src * data: blob:")
	case extRendererOpts.ContentSandbox != "":
		ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'; sandbox "+extRendererOpts.ContentSandbox)
	default:
		ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'")
	}

	err = markup.RenderWithRenderer(rctx, renderer, rendererInput, ctx.Resp)
	if err != nil {
		log.Error("Failed to render file %q: %v", ctx.Repo.TreePath, err)
		http.Error(ctx.Resp, "Failed to render file", http.StatusInternalServerError)
		return
	}
}
