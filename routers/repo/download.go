// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"io"
	"path"

	"code.gitea.io/git"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

// ServeData download file from io.Reader
func ServeData(ctx *context.Context, name string, reader io.Reader) error {
	buf := make([]byte, 1024)
	n, _ := reader.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}

	if !base.IsTextFile(buf) {
		if !base.IsImageFile(buf) {
			ctx.Resp.Header().Set("Content-Disposition", "attachment; filename=\""+path.Base(ctx.Repo.TreePath)+"\"")
			ctx.Resp.Header().Set("Content-Transfer-Encoding", "binary")
		}
	} else if !ctx.QueryBool("render") {
		ctx.Resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	ctx.Resp.Write(buf)
	_, err := io.Copy(ctx.Resp, reader)
	return err
}

// ServeBlob download a git.Blob
func ServeBlob(ctx *context.Context, blob *git.Blob) error {
	dataRc, err := blob.Data()
	if err != nil {
		return err
	}

	return ServeData(ctx, ctx.Repo.TreePath, dataRc)
}

// SingleDownload download a file by repos path
func SingleDownload(ctx *context.Context) {
	blob, err := ctx.Repo.Commit.GetBlobByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.Handle(404, "GetBlobByPath", nil)
		} else {
			ctx.Handle(500, "GetBlobByPath", err)
		}
		return
	}
	if err = ServeBlob(ctx, blob); err != nil {
		ctx.Handle(500, "ServeBlob", err)
	}
}
