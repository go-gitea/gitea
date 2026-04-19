// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"maps"
	"net/http"
	"path"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

// iframeSandboxSafeForSrc reports whether ContentSandbox is strong enough to justify the
// permissive CSP emitted for SrcMethod="src": sandbox must be set and not neutralized by
// allow-same-origin (which would restore parent-origin privileges).
func iframeSandboxSafeForSrc(contentSandbox string) bool {
	return contentSandbox != "" && !slices.Contains(strings.Fields(contentSandbox), "allow-same-origin")
}

// buildIframeCSP emits a permissive CSP for iframes loaded via iframe.src — isolation is
// provided by the iframe sandbox attribute. Renderer-supplied sources are appended per directive.
func buildIframeCSP(additional map[string][]string) string {
	csp := map[string][]string{
		"frame-src":   {"'self'"},
		"script-src":  {"*", "'unsafe-inline'"},
		"style-src":   {"*", "'unsafe-inline'"},
		"default-src": {"*", "data:", "blob:"},
	}
	for name, srcs := range additional {
		csp[name] = append(csp[name], srcs...)
	}
	parts := make([]string, 0, len(csp))
	for _, name := range slices.Sorted(maps.Keys(csp)) {
		parts = append(parts, name+" "+strings.Join(csp[name], " "))
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

	opts := extRenderer.GetExternalRendererOptions()
	switch {
	case opts.SrcMethod == "src":
		// No CSP "sandbox" directive (Firefox refuses same-origin src loading of a sandboxed
		// response); the iframe element's sandbox attribute is our isolation — require a safe one.
		if !iframeSandboxSafeForSrc(opts.ContentSandbox) {
			setting.PanicInDevOrTesting("renderer %q SrcMethod=\"src\" needs sandbox without allow-same-origin (got %q)", renderer.Name(), opts.ContentSandbox)
			log.Error("renderer %q SrcMethod=\"src\" without safe sandbox", renderer.Name())
			http.Error(ctx.Resp, "Renderer misconfigured", http.StatusInternalServerError)
			return
		}
		ctx.Resp.Header().Add("Content-Security-Policy", buildIframeCSP(opts.AdditionalCSPSources))
	case opts.ContentSandbox != "":
		ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'; sandbox "+opts.ContentSandbox)
	default:
		// HINT: PDF-RENDER-SANDBOX: Chrome refuses to render PDFs in a sandboxed context.
		ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'")
	}

	err = markup.RenderWithRenderer(rctx, renderer, rendererInput, ctx.Resp)
	if err != nil {
		log.Error("Failed to render file %q: %v", ctx.Repo.TreePath, err)
		http.Error(ctx.Resp, "Failed to render file", http.StatusInternalServerError)
		return
	}
}
