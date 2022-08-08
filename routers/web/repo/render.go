// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/util"
)

// RenderFile uses an external render to render a file by repos path
func RenderFile(ctx *context.Context) {
	blob, err := ctx.Repo.Commit.GetBlobByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetBlobByPath", err)
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

	treeLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	if ctx.Repo.TreePath != "" {
		treeLink += "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
	}

	renderer := markup.GetRendererByFileName(ctx.Repo.TreePath)
	if renderer == nil {
		ctx.Error(http.StatusBadRequest, "No renderer")
		return
	}

	externalRender, ok := renderer.(markup.ExternalRenderer)
	if !ok {
		ctx.Error(http.StatusBadRequest, "External render only")
		return
	}

	externalCSP := externalRender.ExternalCSP()
	if externalCSP == "" {
		ctx.Error(http.StatusBadRequest, "External render must have valid Content-Security-Header")
		return
	}

	ctx.Resp.Header().Add("Content-Security-Policy", externalCSP)
	ctx.Resp.Header().Add("Content-Type", "text/html")

	if err = markup.RenderDirect(&markup.RenderContext{
		Ctx:              ctx,
		RelativePath:     ctx.Repo.TreePath,
		URLPrefix:        path.Dir(treeLink),
		Metas:            ctx.Repo.Repository.ComposeDocumentMetas(),
		GitRepo:          ctx.Repo.GitRepo,
		InStandalonePage: true,
	}, renderer, dataRc, ctx.Resp); err != nil {
		ctx.ServerError("RenderDirect", err)
		return
	}

	if strings.HasPrefix(ctx.Resp.Header().Get("Content-Type"), "text/html") {
		_, _ = ctx.Resp.Write([]byte(`
<script type='module'>
	// report height to parent
	const fn = () => parent.postMessage({'giteaIframeCmd':'resize','height':document.documentElement.scrollHeight}, "*");
	fn();
	setInterval(fn, 500);
	// make all absolute links open in new window (otherwise they would be blocked by all parents' frame-src)
	document.body.addEventListener('click', (e) => {
		if (e.target.nodeName !== 'A') return;
		const href = e.target.getAttribute('href');
		if (!href.startsWith("//") && !href.includes('://')) return;
		e.target.target = '_blank';
	}, true);
</script>
`))
	}
}
