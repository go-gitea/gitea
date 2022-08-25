// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"errors"
	"net/http"
	"path"
	"time"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/routers/common"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
)

// ServeBlobOrLFS download a git.Blob redirecting to LFS if necessary
func ServeBlobOrLFS(ctx *context.Context, blob *git.Blob, lastModified time.Time) error {
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
		meta, _ := git_model.GetLFSMetaObjectByOid(ctx.Repo.Repository.ID, pointer.Oid)
		if meta == nil {
			if err = dataRc.Close(); err != nil {
				log.Error("ServeBlobOrLFS: Close: %v", err)
			}
			closed = true
			return common.ServeBlob(ctx, blob, lastModified)
		}
		if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+pointer.Oid+`"`) {
			return nil
		}

		if setting.LFS.ServeDirect {
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
		return common.ServeData(ctx, ctx.Repo.TreePath, meta.Size, lfsDataRc)
	}
	if err = dataRc.Close(); err != nil {
		log.Error("ServeBlobOrLFS: Close: %v", err)
	}
	closed = true

	return common.ServeBlob(ctx, blob, lastModified)
}

func getBlobForEntry(ctx *context.Context) (blob *git.Blob, lastModified time.Time) {
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetTreeEntryByPath", err)
		} else {
			ctx.ServerError("GetTreeEntryByPath", err)
		}
		return
	}

	if entry.IsDir() || entry.IsSubModule() {
		ctx.NotFound("getBlobForEntry", nil)
		return
	}

	info, _, err := git.Entries([]*git.TreeEntry{entry}).GetCommitsInfo(ctx, ctx.Repo.Commit, path.Dir("/" + ctx.Repo.TreePath)[1:])
	if err != nil {
		ctx.ServerError("GetCommitsInfo", err)
		return
	}

	if len(info) == 1 {
		// Not Modified
		lastModified = info[0].Commit.Committer.When
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

	if err := common.ServeBlob(ctx, blob, lastModified); err != nil {
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
	if err = common.ServeBlob(ctx, blob, time.Time{}); err != nil {
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
	if err = ServeBlobOrLFS(ctx, blob, time.Time{}); err != nil {
		ctx.ServerError("ServeBlob", err)
	}
}

// Download an archive of a repository
func Download(ctx *context.Context) {
	uri := ctx.Params("*")
	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, uri)
	if err != nil {
		if errors.Is(err, archiver_service.ErrUnknownArchiveFormat{}) {
			ctx.Error(http.StatusBadRequest, err.Error())
		} else {
			ctx.ServerError("archiver_service.NewRequest", err)
		}
		return
	}
	if aReq == nil {
		ctx.Error(http.StatusNotFound)
		return
	}

	archiver, err := repo_model.GetRepoArchiver(ctx, aReq.RepoID, aReq.Type, aReq.CommitID)
	if err != nil {
		ctx.ServerError("models.GetRepoArchiver", err)
		return
	}
	if archiver != nil && archiver.Status == repo_model.ArchiverReady {
		download(ctx, aReq.GetArchiveName(), archiver)
		return
	}

	if err := archiver_service.StartArchive(aReq); err != nil {
		ctx.ServerError("archiver_service.StartArchive", err)
		return
	}

	var times int
	t := time.NewTicker(time.Second * 1)
	defer t.Stop()

	for {
		select {
		case <-graceful.GetManager().HammerContext().Done():
			log.Warn("exit archive download because system stop")
			return
		case <-t.C:
			if times > 20 {
				ctx.ServerError("wait download timeout", nil)
				return
			}
			times++
			archiver, err = repo_model.GetRepoArchiver(ctx, aReq.RepoID, aReq.Type, aReq.CommitID)
			if err != nil {
				ctx.ServerError("archiver_service.StartArchive", err)
				return
			}
			if archiver != nil && archiver.Status == repo_model.ArchiverReady {
				download(ctx, aReq.GetArchiveName(), archiver)
				return
			}
		}
	}
}

func download(ctx *context.Context, archiveName string, archiver *repo_model.RepoArchiver) {
	downloadName := ctx.Repo.Repository.Name + "-" + archiveName

	rPath, err := archiver.RelativePath()
	if err != nil {
		ctx.ServerError("archiver.RelativePath", err)
		return
	}

	if setting.RepoArchive.ServeDirect {
		// If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.RepoArchives.URL(rPath, downloadName)
		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	// If we have matched and access to release or issue
	fr, err := storage.RepoArchives.Open(rPath)
	if err != nil {
		ctx.ServerError("Open", err)
		return
	}
	defer fr.Close()
	ctx.ServeStream(fr, downloadName)
}

// InitiateDownload will enqueue an archival request, as needed.  It may submit
// a request that's already in-progress, but the archiver service will just
// kind of drop it on the floor if this is the case.
func InitiateDownload(ctx *context.Context) {
	uri := ctx.Params("*")
	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, uri)
	if err != nil {
		ctx.ServerError("archiver_service.NewRequest", err)
		return
	}
	if aReq == nil {
		ctx.Error(http.StatusNotFound)
		return
	}

	archiver, err := repo_model.GetRepoArchiver(ctx, aReq.RepoID, aReq.Type, aReq.CommitID)
	if err != nil {
		ctx.ServerError("archiver_service.StartArchive", err)
		return
	}
	if archiver == nil || archiver.Status != repo_model.ArchiverReady {
		if err := archiver_service.StartArchive(aReq); err != nil {
			ctx.ServerError("archiver_service.StartArchive", err)
			return
		}
	}

	var completed bool
	if archiver != nil && archiver.Status == repo_model.ArchiverReady {
		completed = true
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"complete": completed,
	})
}
