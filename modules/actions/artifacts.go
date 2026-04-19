// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"
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
	u, err := storage.ActionsArtifacts.ServeDirectURL(art.StoragePath, art.ArtifactPath, method, &storage.ServeDirectOptions{ContentType: contentType})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func DownloadArtifactV4ServeDirect(ctx *context.Base, art *actions_model.ActionArtifact) bool {
	if !setting.Actions.ArtifactStorage.ServeDirect() {
		return false
	}
	u, err := GetArtifactV4ServeDirectURL(art, ctx.Req.Method)
	if err != nil {
		log.Error("GetArtifactV4ServeDirectURL: %v", err)
		return false
	}
	ctx.Redirect(u, http.StatusFound)
	return true
}

func DownloadArtifactV4ReadStorage(ctx *context.Base, art *actions_model.ActionArtifact) error {
	f, err := storage.ActionsArtifacts.Open(art.StoragePath)
	if err != nil {
		return err
	}
	defer f.Close()
	httplib.ServeUserContentByFile(ctx.Req, ctx.Resp, f, httplib.ServeHeaderOptions{
		Filename:    art.ArtifactPath,
		ContentType: art.ContentEncodingOrType, // v4 guarantees that the field is Content-Type
	})
	return nil
}

func DownloadArtifactV4(ctx *context.Base, art *actions_model.ActionArtifact) error {
	if DownloadArtifactV4ServeDirect(ctx, art) {
		return nil
	}
	return DownloadArtifactV4ReadStorage(ctx, art)
}
