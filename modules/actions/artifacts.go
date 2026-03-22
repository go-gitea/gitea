// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"mime"
	"net/http"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/services/context"
)

// Artifacts using the v4 backend are stored as a single combined zip file per artifact on the backend
// The v4 backend ensures ContentEncoding contains a slash (otherwise this uses application/zip instead of the custom mime type), which is not the case for the old backend
func IsArtifactV4(art *actions_model.ActionArtifact) bool {
	return strings.Contains(art.ContentEncoding, "/")
}

func GetArtifactContentTypeAndDisposition(artifact *actions_model.ActionArtifact) (contentType, contentDisposition string, _ error) {
	contentType = mime.FormatMediaType(artifact.ContentEncoding, nil)
	contentDisposition = public.EncodeContentDisposition(public.ContentDispositionInline, artifact.ArtifactPath)
	if contentType == "" || contentDisposition == "" {
		setting.PanicInDevOrTesting("cannot generate mime headers")
		return "", "", errors.New("cannot generate mime headers")
	}
	return contentType, contentDisposition, nil
}

func GetArtifactV4ServeDirectURL(ctx *context.Base, art *actions_model.ActionArtifact, method string) (string, error) {
	contentType, contentDisposition, err := GetArtifactContentTypeAndDisposition(art)
	if err != nil {
		return "", err
	}
	u, err := storage.ActionsArtifacts.ServeDirectURL(art.StoragePath, art.ArtifactPath, method, &storage.ServeDirectOptions{
		ContentType:        contentType,
		ContentDisposition: contentDisposition,
	})
	if u != nil && err == nil {
		return u.String(), nil
	}
	return "", nil
}

func DownloadArtifactV4ServeDirectOnly(ctx *context.Base, art *actions_model.ActionArtifact) (bool, error) {
	if setting.Actions.ArtifactStorage.ServeDirect() {
		u, err := GetArtifactV4ServeDirectURL(ctx, art, ctx.Req.Method)
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

	contentType, contentDisposition, err := GetArtifactContentTypeAndDisposition(art)
	if err != nil {
		return err
	}

	ctx.Resp.Header().Set("Content-Type", contentType)
	ctx.Resp.Header().Set("Content-Disposition", contentDisposition)
	// HINT: PDF-RENDER-SANDBOX: PDF won't render in sandboxed context, it seems fine to render it inline
	if mediaType, _, err := mime.ParseMediaType(contentType); err != nil || mediaType != "application/pdf" {
		ctx.Resp.Header().Set("Content-Security-Policy", "sandbox; style-src 'unsafe-inline'; default-src 'none';")
	}
	http.ServeContent(ctx.Resp, ctx.Req, art.ArtifactPath, art.CreatedUnix.AsLocalTime(), f)
	return nil
}

func DownloadArtifactV4(ctx *context.Base, art *actions_model.ActionArtifact) error {
	ok, err := DownloadArtifactV4ServeDirectOnly(ctx, art)
	if ok || err != nil {
		return err
	}
	return DownloadArtifactV4Fallback(ctx, art)
}
