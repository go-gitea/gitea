// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"path"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/services/context"
)

// buildIframeCSP returns the CSP for an iframe response loaded via iframe.src. The baseline is
// permissive (isolation is provided by the iframe sandbox attribute); renderer-specific sources
// are appended to the named directive.
func buildIframeCSP(additional map[string][]string) string {
	directives := []struct {
		name string
		srcs []string
	}{
		{"frame-src", []string{"'self'"}},
		{"script-src", []string{"*", "'unsafe-inline'"}},
		{"style-src", []string{"*", "'unsafe-inline'"}},
		{"default-src", []string{"*", "data:", "blob:"}},
	}
	parts := make([]string, 0, len(directives))
	for _, d := range directives {
		srcs := slices.Concat(d.srcs, additional[d.name])
		parts = append(parts, d.name+" "+strings.Join(srcs, " "))
	}
	return strings.Join(parts, "; ")
}

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

	extRendererOpts := extRenderer.GetExternalRendererOptions()
	if extRendererOpts.SrcMethod == "src" {
		// Iframe is loaded via "src", so the response must NOT carry the CSP "sandbox" directive
		// (Firefox refuses same-origin src loading of a sandboxed response). Isolation comes from
		// the iframe's sandbox attribute. Renderers can append additional sources (e.g.
		// 'wasm-unsafe-eval' for asciinema-player) without widening the main site CSP.
		ctx.Resp.Header().Add("Content-Security-Policy", buildIframeCSP(extRendererOpts.AdditionalCSPSources))
	} else if extRendererOpts.ContentSandbox != "" {
		ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'; sandbox "+extRendererOpts.ContentSandbox)
	} else {
		// HINT: PDF-RENDER-SANDBOX: PDF won't render in sandboxed context — Chrome blocks the PDF
		// rendering when sandboxed, even if all "allow-*" are set; renderers opt out of sandboxing
		// by leaving ContentSandbox empty.
		ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'")
	}

	err = markup.RenderWithRenderer(rctx, renderer, rendererInput, ctx.Resp)
	if err != nil {
		log.Error("Failed to render file %q: %v", ctx.Repo.TreePath, err)
		http.Error(ctx.Resp, "Failed to render file", http.StatusInternalServerError)
		return
	}
}
