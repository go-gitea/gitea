// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"io"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
)

// ServeBlob download a git.Blob
func ServeBlob(ctx *context.Context, blob *git.Blob, lastModified time.Time) error {
	if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
		return nil
	}

	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}
	defer func() {
		if err = dataRc.Close(); err != nil {
			log.Error("ServeBlob: Close: %v", err)
		}
	}()

	httplib.ServeContentByReader(ctx.Req, ctx.Resp, ctx.Repo.TreePath, blob.Size(), dataRc)
	return nil
}

func ServeContentByReader(ctx *context.Context, filePath string, size int64, reader io.Reader) {
	httplib.ServeContentByReader(ctx.Req, ctx.Resp, filePath, size, reader)
}

func ServeContentByReadSeeker(ctx *context.Context, filePath string, modTime time.Time, reader io.ReadSeeker) {
	httplib.ServeContentByReadSeeker(ctx.Req, ctx.Resp, filePath, modTime, reader)
}
