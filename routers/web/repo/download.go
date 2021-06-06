// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/routers/common"
)

// ServeBlobOrLFS download a git.Blob redirecting to LFS if necessary
func ServeBlobOrLFS(ctx *context.Context, blob *git.Blob) error {
	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`) {
		return nil
	}

	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if closed {
			return
		}
		if err = dataRc.Close(); err != nil {
			log.Error("ServeBlobOrLFS: Close: %v", err)
		}
	}()

	pointer, _ := lfs.ReadPointer(dataRc)
	if pointer.IsValid() {
		meta, _ := ctx.Repo.Repository.GetLFSMetaObjectByOid(pointer.Oid)
		if meta == nil {
			if err = dataRc.Close(); err != nil {
				log.Error("ServeBlobOrLFS: Close: %v", err)
			}
			closed = true
			return common.ServeBlob(ctx, blob)
		}
		if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+pointer.Oid+`"`) {
			return nil
		}
		lfsDataRc, err := lfs.ReadMetaObject(meta.Pointer)
		if err != nil {
			return err
		}
		defer func() {
			if err = lfsDataRc.Close(); err != nil {
				log.Error("ServeBlobOrLFS: Close: %v", err)
			}
		}()
		return common.ServeData(ctx, ctx.Repo.TreePath, meta.Size, lfsDataRc)
	}
	if err = dataRc.Close(); err != nil {
		log.Error("ServeBlobOrLFS: Close: %v", err)
	}
	closed = true

	return common.ServeBlob(ctx, blob)
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
	if err = common.ServeBlob(ctx, blob); err != nil {
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
	if err = common.ServeBlob(ctx, blob); err != nil {
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
