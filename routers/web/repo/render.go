// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"

	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
)

// RenderFile renders a file by repos path
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

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	st := typesniffer.DetectContentType(buf)
	isTextFile := st.IsText()

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))

	if markupType := markup.Type(blob.Name()); markupType == "" {
		if isTextFile {
			_, err = io.Copy(ctx.Resp, rd)
			if err != nil {
				ctx.ServerError("Copy", err)
			}
			return
		}
		ctx.Error(http.StatusInternalServerError, "Unsupported file type render")
		return
	}

	treeLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	if ctx.Repo.TreePath != "" {
		treeLink += "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
	}

	var allowSameOriginStr string

	renderer, err := markup.GetRenderer("", ctx.Repo.TreePath)
	if err != nil {
		ctx.ServerError("GetRenderer", err)
		return
	}

	if r, ok := renderer.(markup.ExternalRenderer); ok && r.AllowSameOrigin() {
		allowSameOriginStr = " allow-same-origin"
	}

	ctx.Resp.Header().Add("Content-Security-Policy", fmt.Sprintf("frame-src 'self'; sandbox%s allow-scripts", allowSameOriginStr))

	if err = markup.RenderDirect(&markup.RenderContext{
		Ctx:          ctx,
		RelativePath: ctx.Repo.TreePath,
		URLPrefix:    path.Dir(treeLink),
		Metas:        ctx.Repo.Repository.ComposeDocumentMetas(),
		GitRepo:      ctx.Repo.GitRepo,
	}, renderer, rd, ctx.Resp); err != nil {
		ctx.ServerError("RenderDirect", err)
		return
	}
}
