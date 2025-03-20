// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/services/context"
)

// Artifacts using the v4 backend are stored as a single combined zip file per artifact on the backend
// The v4 backend ensures ContentEncoding is set to "application/zip", which is not the case for the old backend
func IsArtifactV4(art *actions_model.ActionArtifact) bool {
	return art.ArtifactName+".zip" == art.ArtifactPath && art.ContentEncoding == "application/zip"
}

func DownloadArtifactV4ServeDirectOnly(ctx *context.Base, art *actions_model.ActionArtifact) (bool, error) {
	if setting.Actions.ArtifactStorage.ServeDirect() {
		u, err := storage.ActionsArtifacts.URL(art.StoragePath, art.ArtifactPath, nil)
		if u != nil && err == nil {
			ctx.Redirect(u.String(), http.StatusFound)
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
	http.ServeContent(ctx.Resp, ctx.Req, art.ArtifactName+".zip", art.CreatedUnix.AsLocalTime(), f)
	return nil
}

func DownloadArtifactV4(ctx *context.Base, art *actions_model.ActionArtifact) error {
	ok, err := DownloadArtifactV4ServeDirectOnly(ctx, art)
	if ok || err != nil {
		return err
	}
	return DownloadArtifactV4Fallback(ctx, art)
}
