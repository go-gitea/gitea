// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	goproxy_service "gitea.dev/services/packages/goproxy"

	"golang.org/x/mod/module"
)

// Global routes serve any module path from /api/packages/go.

func GlobalEnumeratePackageVersions(ctx *context.Context) {
	modulePath := ctx.PathParam("name")
	repo, local, ok := globalLocalRepo(ctx, modulePath)
	if !ok {
		return
	}
	if !local {
		proxyThroughList(ctx, modulePath, "", "")
		return
	}

	if err := serveRepositoryModule(ctx, repo, "", ""); err != nil {
		writeProxyError(ctx, err)
	}
}

func GlobalPackageVersionMetadata(ctx *context.Context) {
	modulePath := ctx.PathParam("name")
	version := ctx.PathParam("version")

	repo, local, ok := globalLocalRepo(ctx, modulePath)
	if !ok {
		return
	}
	if !local {
		proxyThroughList(ctx, modulePath, version, "info")
		return
	}

	if err := serveRepositoryModule(ctx, repo, version, "info"); err != nil {
		writeProxyError(ctx, err)
	}
}

func GlobalPackageVersionGoModContent(ctx *context.Context) {
	modulePath := ctx.PathParam("name")
	version := ctx.PathParam("version")

	repo, local, ok := globalLocalRepo(ctx, modulePath)
	if !ok {
		return
	}
	if !local {
		proxyThroughList(ctx, modulePath, version, "mod")
		return
	}

	if err := serveRepositoryModule(ctx, repo, version, "mod"); err != nil {
		writeProxyError(ctx, err)
	}
}

func GlobalDownloadPackageFile(ctx *context.Context) {
	modulePath := ctx.PathParam("name")
	version := ctx.PathParam("version")

	repo, local, ok := globalLocalRepo(ctx, modulePath)
	if !ok {
		return
	}
	if !local {
		proxyThroughList(ctx, modulePath, version, "zip")
		return
	}

	if err := serveRepositoryModule(ctx, repo, version, "zip"); err != nil {
		writeProxyError(ctx, err)
	}
}

func globalLocalRepo(ctx *context.Context, modulePath string) (*goproxy_service.Repository, bool, bool) {
	if err := module.CheckPath(modulePath); err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return nil, false, false
	}

	repo, local, err := goproxy_service.ResolveRepository(ctx, modulePath)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return nil, false, false
	}
	if !local {
		return nil, false, true
	}
	if repo == nil {
		apiError(ctx, http.StatusNotFound, goproxy_service.ErrNotFound)
		return nil, false, false
	}

	scope, _ := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
	if err := repo.CheckAccess(ctx, ctx.Doer, scope); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, goproxy_service.ErrNotFound) || errors.Is(err, util.ErrNotExist) {
			status = http.StatusNotFound
		}
		apiError(ctx, status, err)
		return nil, false, false
	}

	return repo, true, true
}

func serveRepositoryModule(ctx *context.Context, repo *goproxy_service.Repository, version, file string) *proxyError {
	if file == "" {
		versions, err := repo.ListVersions(ctx)
		if err != nil {
			return &proxyError{status: http.StatusInternalServerError, body: err.Error()}
		}

		ctx.Resp.Header().Set("Content-Type", "text/plain;charset=utf-8")
		for _, v := range versions {
			fmt.Fprintln(ctx.Resp, v)
		}
		return nil
	}

	v, err := repo.ResolveVersion(ctx, version)
	if err != nil {
		return versionProxyError(err)
	}

	switch file {
	case "info":
		ctx.JSON(http.StatusOK, struct {
			Version string    `json:"Version"`
			Time    time.Time `json:"Time"`
		}{
			Version: v.Version,
			Time:    v.Time,
		})
		return nil
	case "mod":
		goMod, err := v.GoMod(ctx)
		if err != nil {
			return versionProxyError(err)
		}
		ctx.PlainText(http.StatusOK, string(goMod))
		return nil
	case "zip":
		tmpFile, cleanup, err := setting.AppDataTempDir("goproxy").CreateTempFileRandom("module-*.zip")
		if err != nil {
			return &proxyError{status: http.StatusInternalServerError, body: err.Error()}
		}
		defer cleanup()

		if err := v.CreateZip(ctx, tmpFile); err != nil {
			return versionProxyError(err)
		}

		if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
			return &proxyError{status: http.StatusInternalServerError, body: err.Error()}
		}

		ctx.ServeContent(tmpFile, context.ServeHeaderOptions{
			ContentType:  "application/zip",
			Filename:     v.Version + ".zip",
			LastModified: v.Time,
		})
		return nil
	}

	return &proxyError{status: http.StatusInternalServerError, body: "unknown Go proxy file type"}
}

func serveCachedGoProxyObject(ctx *context.Context, obj storage.Object, filename, contentType string) {
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	httplib.ServeUserContentByReader(ctx.Req, ctx.Resp, info.Size(), obj, httplib.ServeHeaderOptions{
		ContentType:  contentType,
		Filename:     filename,
		LastModified: info.ModTime(),
	})
}

func goProxyContentType(file string) string {
	switch file {
	case "info":
		return "application/json"
	case "mod":
		return "text/plain;charset=utf-8"
	case "zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

func goProxyCacheKey(modulePath, version, file string) string {
	sum := sha256.Sum256([]byte(modulePath + "@" + version + "." + file))
	encoded := hex.EncodeToString(sum[:])
	return path.Join("goproxy", encoded[:2], encoded[2:4], encoded)
}

func upstreamGet(ctx *context.Context, upstreamURL string) (*http.Response, error) {
	return httplib.NewRequest(upstreamURL, http.MethodGet).SetContext(ctx).Response()
}

func forwardUpstreamResponse(ctx *context.Context, resp *http.Response) {
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		ctx.Resp.Header().Set("Content-Type", contentType)
	}
	ctx.Resp.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(ctx.Resp, resp.Body)
}

func buildUpstreamURL(baseURL, modulePath, version, file string) (string, error) {
	escapedPath, err := module.EscapePath(modulePath)
	if err != nil {
		return "", err
	}

	base := strings.TrimRight(baseURL, "/") + "/" + escapedPath
	switch file {
	case "":
		return base + "/@v/list", nil
	case "latest":
		return base + "/@latest", nil
	case "info":
		if version == "latest" {
			return base + "/@latest", nil
		}
	case "mod", "zip":
		if version == "latest" {
			return "", goproxy_service.ErrInvalidVersion
		}
	}

	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		return "", err
	}
	return base + "/@v/" + escapedVersion + "." + file, nil
}
