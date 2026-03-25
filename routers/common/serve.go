// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"io"
	"path"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
)

// ServeBlob download a git.Blob
func ServeBlob(ctx *context.Base, repo *repo_model.Repository, filePath string, blob *git.Blob, lastModified *time.Time) error {
	if httpcache.HandleGenericETagPrivateCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
		return nil
	}

	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}
	defer dataRc.Close()

	if err = repo.LoadOwner(ctx); err != nil {
		return err
	}
	httplib.ServeContentByReader(ctx.Req, ctx.Resp, blob.Size(), dataRc, httplib.ServeHeaderOptions{
		Filename:      path.Base(filePath),
		CacheIsPublic: !repo.IsPrivate && repo.Owner.Visibility == structs.VisibleTypePublic,
		CacheDuration: setting.StaticCacheTime,
	})
	return nil
}

func ServeContentByReadSeeker(ctx *context.Base, filePath string, modTime *time.Time, reader io.ReadSeeker) {
	httplib.ServeContentByReadSeeker(ctx.Req, ctx.Resp, modTime, reader, httplib.ServeHeaderOptions{Filename: path.Base(filePath)})
}
