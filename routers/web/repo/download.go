// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"path"
	"time"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/routers/common"
)

// ServeBlobOrLFS download a git.Blob redirecting to LFS if necessary
func ServeBlobOrLFS(ctx *context.Context, blob *git.Blob, lastModified *time.Time) error {
	if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
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
		meta, _ := git_model.GetLFSMetaObjectByOid(ctx, ctx.Repo.Repository.ID, pointer.Oid)
		if meta == nil {
			if err = dataRc.Close(); err != nil {
				log.Error("ServeBlobOrLFS: Close: %v", err)
			}
			closed = true
			return common.ServeBlob(ctx.Base, ctx.Repo.TreePath, blob, lastModified)
		}
		if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+pointer.Oid+`"`) {
			return nil
		}

		if setting.LFS.Storage.MinioConfig.ServeDirect {
			// If we have a signed url (S3, object storage), redirect to this directly.
			u, err := storage.LFS.URL(pointer.RelativePath(), blob.Name())
			if u != nil && err == nil {
				ctx.Redirect(u.String())
				return nil
			}
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
		common.ServeContentByReadSeeker(ctx.Base, ctx.Repo.TreePath, lastModified, lfsDataRc)
		return nil
	}
	if err = dataRc.Close(); err != nil {
		log.Error("ServeBlobOrLFS: Close: %v", err)
	}
	closed = true

	return common.ServeBlob(ctx.Base, ctx.Repo.TreePath, blob, lastModified)
}

func getBlobForEntry(ctx *context.Context) (blob *git.Blob, lastModified *time.Time) {
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetTreeEntryByPath", err)
		} else {
			ctx.ServerError("GetTreeEntryByPath", err)
		}
		return nil, nil
	}

	if entry.IsDir() || entry.IsSubModule() {
		ctx.NotFound("getBlobForEntry", nil)
		return nil, nil
	}

	info, _, err := git.Entries([]*git.TreeEntry{entry}).GetCommitsInfo(ctx, ctx.Repo.Commit, path.Dir("/" + ctx.Repo.TreePath)[1:])
	if err != nil {
		ctx.ServerError("GetCommitsInfo", err)
		return nil, nil
	}

	if len(info) == 1 {
		// Not Modified
		lastModified = &info[0].Commit.Committer.When
	}
	blob = entry.Blob()

	return blob, lastModified
}

// SingleDownload download a file by repos path
func SingleDownload(ctx *context.Context) {
	blob, lastModified := getBlobForEntry(ctx)
	if blob == nil {
		return
	}

	if err := common.ServeBlob(ctx.Base, ctx.Repo.TreePath, blob, lastModified); err != nil {
		ctx.ServerError("ServeBlob", err)
	}
}

// SingleDownloadOrLFS download a file by repos path redirecting to LFS if necessary
func SingleDownloadOrLFS(ctx *context.Context) {
	blob, lastModified := getBlobForEntry(ctx)
	if blob == nil {
		return
	}

	if err := ServeBlobOrLFS(ctx, blob, lastModified); err != nil {
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
	if err = common.ServeBlob(ctx.Base, ctx.Repo.TreePath, blob, nil); err != nil {
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
	if err = ServeBlobOrLFS(ctx, blob, nil); err != nil {
		ctx.ServerError("ServeBlob", err)
	}
}
