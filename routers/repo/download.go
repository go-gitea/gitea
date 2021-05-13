// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// ServeData download file from io.Reader
func ServeData(ctx *context.Context, name string, size int64, reader io.Reader) error {
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		return err
	}
	if n >= 0 {
		buf = buf[:n]
	}

	ctx.Resp.Header().Set("Cache-Control", "public,max-age=86400")

	if size >= 0 {
		ctx.Resp.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	} else {
		log.Error("ServeData called to serve data: %s with size < 0: %d", name, size)
	}
	name = path.Base(name)

	// Google Chrome dislike commas in filenames, so let's change it to a space
	name = strings.ReplaceAll(name, ",", " ")

	if base.IsTextFile(buf) || ctx.QueryBool("render") {
		cs, err := charset.DetectEncoding(buf)
		if err != nil {
			log.Error("Detect raw file %s charset failed: %v, using by default utf-8", name, err)
			cs = "utf-8"
		}
		ctx.Resp.Header().Set("Content-Type", "text/plain; charset="+strings.ToLower(cs))
	} else if base.IsImageFile(buf) || base.IsPDFFile(buf) {
		ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, name))
		ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		if base.IsSVGImageFile(buf) {
			ctx.Resp.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
			ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
			ctx.Resp.Header().Set("Content-Type", base.SVGMimeType)
		}
	} else {
		ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		if setting.MimeTypeMap.Enabled {
			fileExtension := strings.ToLower(filepath.Ext(name))
			if mimetype, ok := setting.MimeTypeMap.Map[fileExtension]; ok {
				ctx.Resp.Header().Set("Content-Type", mimetype)
			}
		}
	}

	_, err = ctx.Resp.Write(buf)
	if err != nil {
		return err
	}
	_, err = io.Copy(ctx.Resp, reader)
	return err
}

// ServeBlob download a git.Blob
func ServeBlob(ctx *context.Context, blob *git.Blob) error {
	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`) {
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

	return ServeData(ctx, ctx.Repo.TreePath, blob.Size(), dataRc)
}

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
			return ServeBlob(ctx, blob)
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
		return ServeData(ctx, ctx.Repo.TreePath, meta.Size, lfsDataRc)
	}
	if err = dataRc.Close(); err != nil {
		log.Error("ServeBlobOrLFS: Close: %v", err)
	}
	closed = true

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
