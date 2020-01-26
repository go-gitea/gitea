// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"io"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
)

// ServeData download file from io.Reader
func ServeData(ctx *context.Context, name string, reader io.Reader) error {
	buf := make([]byte, 1024)
	n, _ := reader.Read(buf)
	if n >= 0 {
		buf = buf[:n]
	}

	ctx.Resp.Header().Set("Cache-Control", "public,max-age=86400")
	name = path.Base(name)

	// Google Chrome dislike commas in filenames, so let's change it to a space
	name = strings.Replace(name, ",", " ", -1)

	if base.IsTextFile(buf) || ctx.QueryBool("render") {
		cs, err := charset.DetectEncoding(buf)
		if err != nil {
			log.Error("Detect raw file %s charset failed: %v, using by default utf-8", name, err)
			cs = "utf-8"
		}
		ctx.Resp.Header().Set("Content-Type", "text/plain; charset="+strings.ToLower(cs))
	} else if base.IsImageFile(buf) || base.IsPDFFile(buf) {
		ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, name))
	} else {
		ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	}

	_, err := ctx.Resp.Write(buf)
	if err != nil {
		return err
	}
	_, err = io.Copy(ctx.Resp, reader)
	return err
}

// ServeBlob download a git.Blob
func ServeBlob(ctx *context.Context, blob *git.Blob) error {
	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}
	defer func() {
		if err = dataRc.Close(); err != nil {
			log.Error("ServeBlob: Close: %v", err)
		}
	}()

	return ServeData(ctx, ctx.Repo.TreePath, dataRc)
}

// ServeBlobOrLFS download a git.Blob redirecting to LFS if necessary
func ServeBlobOrLFS(ctx *context.Context, blob *git.Blob) error {
	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}
	defer func() {
		if err = dataRc.Close(); err != nil {
			log.Error("ServeBlobOrLFS: Close: %v", err)
		}
	}()

	if meta, _ := lfs.ReadPointerFile(dataRc); meta != nil {
		meta, _ = ctx.Repo.Repository.GetLFSMetaObjectByOid(meta.Oid)
		if meta == nil {
			return ServeBlob(ctx, blob)
		}
		lfsDataRc, err := lfs.ReadMetaObject(meta)
		if err != nil {
			return err
		}
		defer func() {
			if err = lfsDataRc.Close(); err != nil {
				log.Error("ServeBlobOrLFS: Close: %v", err)
			}
		}()
		return ServeData(ctx, ctx.Repo.TreePath, lfsDataRc)
	}

	return ServeBlob(ctx, blob)
}

// SingleDownload download a file by repos path
func SingleDownload(ctx *context.Context) {
	blob, err := ctx.Repo.Commit.GetBlobByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetBlobByPath", nil)
		} else {
			ctx.ServerError("GetBlobByPath", err)
		}
		return
	}
	if err = ServeBlob(ctx, blob); err != nil {
		ctx.ServerError("ServeBlob", err)
	}
}

// SingleDownloadOrLFS download a file by repos path redirecting to LFS if necessary
func SingleDownloadOrLFS(ctx *context.Context) {
	blob, err := ctx.Repo.Commit.GetBlobByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetBlobByPath", nil)
		} else {
			ctx.ServerError("GetBlobByPath", err)
		}
		return
	}
	if err = ServeBlobOrLFS(ctx, blob); err != nil {
		ctx.ServerError("ServeBlobOrLFS", err)
	}
}

// DownloadByID download a file by sha1 ID
func DownloadByID(ctx *context.Context) {
	blob, err := ctx.Repo.GitRepo.GetBlob(ctx.Params("sha"))
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetBlob", nil)
		} else {
			ctx.ServerError("GetBlob", err)
		}
		return
	}
	if err = ServeBlob(ctx, blob); err != nil {
		ctx.ServerError("ServeBlob", err)
	}
}

// DownloadByIDOrLFS download a file by sha1 ID taking account of LFS
func DownloadByIDOrLFS(ctx *context.Context) {
	blob, err := ctx.Repo.GitRepo.GetBlob(ctx.Params("sha"))
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetBlob", nil)
		} else {
			ctx.ServerError("GetBlob", err)
		}
		return
	}
	if err = ServeBlobOrLFS(ctx, blob); err != nil {
		ctx.ServerError("ServeBlob", err)
	}
}
