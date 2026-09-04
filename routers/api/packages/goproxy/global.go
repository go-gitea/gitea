// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	stdcontext "context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/globallock"
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
		proxyUpstream(ctx, modulePath, "", "")
		return
	}

	versions, err := repo.ListVersions(ctx)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Resp.Header().Set("Content-Type", "text/plain;charset=utf-8")
	for _, version := range versions {
		fmt.Fprintln(ctx.Resp, version)
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
		proxyUpstream(ctx, modulePath, version, "info")
		return
	}

	v, err := repo.ResolveVersion(ctx, version)
	if err != nil {
		globalVersionError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, struct {
		Version string    `json:"Version"`
		Time    time.Time `json:"Time"`
	}{
		Version: v.Version,
		Time:    v.Time,
	})
}

func GlobalPackageVersionGoModContent(ctx *context.Context) {
	modulePath := ctx.PathParam("name")
	version := ctx.PathParam("version")

	repo, local, ok := globalLocalRepo(ctx, modulePath)
	if !ok {
		return
	}
	if !local {
		proxyUpstream(ctx, modulePath, version, "mod")
		return
	}

	v, err := repo.ResolveVersion(ctx, version)
	if err != nil {
		globalVersionError(ctx, err)
		return
	}

	goMod, err := v.GoMod(ctx)
	if err != nil {
		globalVersionError(ctx, err)
		return
	}

	ctx.PlainText(http.StatusOK, string(goMod))
}

func GlobalDownloadPackageFile(ctx *context.Context) {
	modulePath := ctx.PathParam("name")
	version := ctx.PathParam("version")

	repo, local, ok := globalLocalRepo(ctx, modulePath)
	if !ok {
		return
	}
	if !local {
		proxyUpstream(ctx, modulePath, version, "zip")
		return
	}

	v, err := repo.ResolveVersion(ctx, version)
	if err != nil {
		globalVersionError(ctx, err)
		return
	}

	tmpFile, cleanup, err := setting.AppDataTempDir("goproxy").CreateTempFileRandom("module-*.zip")
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer cleanup()

	if err := v.CreateZip(ctx, tmpFile); err != nil {
		globalVersionError(ctx, err)
		return
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.ServeContent(tmpFile, context.ServeHeaderOptions{
		ContentType:  "application/zip",
		Filename:     v.Version + ".zip",
		LastModified: v.Time,
	})
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

func globalVersionError(ctx *context.Context, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, goproxy_service.ErrNotFound) || errors.Is(err, goproxy_service.ErrGoModNotFound) || errors.Is(err, util.ErrNotExist) {
		status = http.StatusNotFound
	} else if errors.Is(err, goproxy_service.ErrInvalidVersion) || errors.Is(err, goproxy_service.ErrGoModMismatch) || errors.Is(err, goproxy_service.ErrGoModTooLarge) {
		status = http.StatusBadRequest
	}
	apiError(ctx, status, err)
}

func proxyUpstream(ctx *context.Context, modulePath, version, file string) {
	if setting.Packages.GoProxyURL == "" {
		apiError(ctx, http.StatusNotFound, goproxy_service.ErrNotFound)
		return
	}

	upstreamURL, err := buildUpstreamURL(modulePath, version, file)
	if err != nil {
		globalVersionError(ctx, err)
		return
	}

	if (file == "info" && version != "latest") || file == "mod" || file == "zip" {
		proxyUpstreamImmutable(ctx, upstreamURL, modulePath, version, file)
		return
	}

	resp, err := upstreamGet(ctx, upstreamURL)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer resp.Body.Close()

	forwardUpstreamResponse(ctx, resp)
}

func proxyUpstreamImmutable(ctx *context.Context, upstreamURL, modulePath, version, file string) {
	if version == "latest" {
		// The Go command requests the latest version through @latest; the
		// per-owner registry also accepts latest.info/mod/zip, but that is
		// not part of the upstream proxy protocol.
		apiError(ctx, http.StatusNotFound, goproxy_service.ErrNotFound)
		return
	}

	cacheKey := goProxyCacheKey(modulePath, version, file)
	if obj, err := storage.Packages.Open(cacheKey); err == nil {
		serveCachedGoProxyObject(ctx, obj, version+"."+file, goProxyContentType(file))
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	handled := false
	err := globallock.LockAndDo(ctx, "goproxy-cache:"+cacheKey, func(stdcontext.Context) error {
		if obj, err := storage.Packages.Open(cacheKey); err == nil {
			_ = obj.Close()
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		resp, err := upstreamGet(ctx, upstreamURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			forwardUpstreamResponse(ctx, resp)
			handled = true
			return nil
		}

		if err := storage.SaveFrom(storage.Packages, cacheKey, func(w io.Writer) error {
			_, err := io.Copy(w, resp.Body)
			return err
		}); err != nil {
			_ = storage.Packages.Delete(cacheKey)
			return err
		}
		return nil
	})
	if handled {
		return
	}
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	obj, err := storage.Packages.Open(cacheKey)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	serveCachedGoProxyObject(ctx, obj, version+"."+file, goProxyContentType(file))
}

func serveCachedGoProxyObject(ctx *context.Context, obj storage.Object, filename, contentType string) {
	defer obj.Close()

	lastModified := time.Time{}
	if info, err := obj.Stat(); err == nil {
		lastModified = info.ModTime()
	}

	ctx.ServeContent(obj, context.ServeHeaderOptions{
		ContentType:  contentType,
		Filename:     filename,
		LastModified: lastModified,
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

func buildUpstreamURL(modulePath, version, file string) (string, error) {
	escapedPath, err := module.EscapePath(modulePath)
	if err != nil {
		return "", err
	}

	base := strings.TrimRight(setting.Packages.GoProxyURL, "/") + "/" + escapedPath
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
