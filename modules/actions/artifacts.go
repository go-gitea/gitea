// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"
	"path"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/services/context"
)

// IsArtifactV4 detects whether the artifact is likely from v4.
// V4 backend stores the files as a single combined zip file per artifact, and ensures ContentEncoding contains a slash
// (otherwise this uses application/zip instead of the custom mime type), which is not the case for the old backend.
func IsArtifactV4(art *actions_model.ActionArtifact) bool {
	return strings.Contains(art.ContentEncodingOrType, "/")
}

func GetArtifactV4ServeDirectURL(art *actions_model.ActionArtifact, method string) (string, error) {
	contentType := art.ContentEncodingOrType
	contentDisposition := httplib.EncodeContentDisposition(httplib.ContentDispositionInline, path.Base(art.ArtifactPath))
	u, err := storage.ActionsArtifacts.ServeDirectURL(art.StoragePath, art.ArtifactPath, method, &storage.ServeDirectOptions{
		ContentType:        contentType,
		ContentDisposition: contentDisposition,
	})
	if err != nil {
		log.Error("GetArtifactV4ServeDirectURL failed with error: %v", err)
		return "", nil
	}
	return u.String(), nil
}

func DownloadArtifactV4ServeDirectOnly(ctx *context.Base, art *actions_model.ActionArtifact) (bool, error) {
	if setting.Actions.ArtifactStorage.ServeDirect() {
		u, err := GetArtifactV4ServeDirectURL(art, ctx.Req.Method)
		if u != "" && err == nil {
			ctx.Redirect(u, http.StatusFound)
			return true, nil
		}
	}
	return false, nil
}

func DownloadArtifactV4Fallback(ctx *context.Base, art *actions_model.ActionArtifact) error {
	f, err := storage.ActionsArtifacts.Open(art.StoragePath)
	if err != nil {
		return err
	}
	defer f.Close()

	contentType := art.ContentEncodingOrType
	contentLength := int64(-1) // do we know the content length (by artifact)?
	httplib.ServeContentByReader(ctx.Req, ctx.Resp, contentLength, f, httplib.ServeHeaderOptions{
		Filename:           path.Base(art.ArtifactPath),
		ContentType:        contentType,
		ContentDisposition: httplib.ContentDispositionInline,
	})
	return nil
}

func DownloadArtifactV4(ctx *context.Base, art *actions_model.ActionArtifact) error {
	ok, err := DownloadArtifactV4ServeDirectOnly(ctx, art)
	if ok || err != nil {
		return err
	}
	return DownloadArtifactV4Fallback(ctx, art)
}
