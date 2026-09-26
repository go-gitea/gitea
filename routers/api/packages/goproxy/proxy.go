// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	stdcontext "context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"gitea.dev/modules/globallock"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	goproxy_service "gitea.dev/services/packages/goproxy"
)

type proxyKind uint8

const (
	proxyKindURL proxyKind = iota
	proxyKindDirect
	proxyKindOff
)

type proxySpec struct {
	kind            proxyKind
	url             string
	fallbackOnError bool
}

type proxyError struct {
	status int
	body   string
}

func (e *proxyError) Error() string {
	return e.body
}

func writeProxyError(ctx *context.Context, err *proxyError) {
	if err == nil {
		return
	}
	if err.status == 0 {
		err.status = http.StatusInternalServerError
	}
	apiError(ctx, err.status, err)
}

func versionProxyError(err error) *proxyError {
	status := http.StatusInternalServerError
	if errors.Is(err, goproxy_service.ErrNotFound) || errors.Is(err, goproxy_service.ErrGoModNotFound) || errors.Is(err, util.ErrNotExist) {
		status = http.StatusNotFound
	} else if errors.Is(err, goproxy_service.ErrInvalidVersion) || errors.Is(err, goproxy_service.ErrGoModMismatch) || errors.Is(err, goproxy_service.ErrGoModTooLarge) {
		status = http.StatusBadRequest
	}
	return &proxyError{status: status, body: err.Error()}
}

func parseGoProxyList(raw string) ([]proxySpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty GOPROXY list")
	}

	var specs []proxySpec
	for raw != "" {
		item := raw
		fallbackOnError := false
		if i := strings.IndexAny(raw, ",|"); i >= 0 {
			item = raw[:i]
			fallbackOnError = raw[i] == '|'
			raw = raw[i+1:]
		} else {
			raw = ""
		}

		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		switch item {
		case "off":
			specs = append(specs, proxySpec{kind: proxyKindOff, fallbackOnError: fallbackOnError})
			return specs, nil
		case "direct":
			specs = append(specs, proxySpec{kind: proxyKindDirect, fallbackOnError: fallbackOnError})
		case "noproxy":
			// noproxy is a client-side concept, so skip it when parsing a server-side list.
			continue
		default:
			u, err := url.Parse(item)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return nil, fmt.Errorf("invalid GOPROXY URL %q", item)
			}
			specs = append(specs, proxySpec{kind: proxyKindURL, url: item, fallbackOnError: fallbackOnError})
		}
	}

	if len(specs) == 0 {
		return nil, errors.New("empty GOPROXY list")
	}
	return specs, nil
}

func proxyThroughList(ctx *context.Context, modulePath, version, file string) {
	specs, err := parseGoProxyList(setting.Packages.GoProxyURL)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	var lastErr *proxyError
	for _, spec := range specs {
		err := serveProxySpec(ctx, spec, modulePath, version, file)
		if err == nil {
			return
		}
		lastErr = err

		if spec.fallbackOnError {
			continue
		}
		if err.status == http.StatusNotFound || err.status == http.StatusGone {
			continue
		}
		break
	}

	if lastErr == nil {
		lastErr = &proxyError{status: http.StatusNotFound, body: "module not found"}
	}
	writeProxyError(ctx, lastErr)
}

func serveProxySpec(ctx *context.Context, spec proxySpec, modulePath, version, file string) *proxyError {
	switch spec.kind {
	case proxyKindOff:
		return &proxyError{status: http.StatusNotFound, body: "module proxy is off"}
	case proxyKindDirect:
		return serveDirectModule(ctx, modulePath, version, file)
	default:
		return serveURLProxy(ctx, spec.url, modulePath, version, file)
	}
}

func serveURLProxy(ctx *context.Context, upstream, modulePath, version, file string) *proxyError {
	upstreamURL, err := buildUpstreamURL(upstream, modulePath, version, file)
	if err != nil {
		return &proxyError{status: http.StatusBadRequest, body: err.Error()}
	}

	if (file == "info" && version != "latest") || file == "mod" || file == "zip" {
		return serveURLProxyImmutable(ctx, upstreamURL, modulePath, version, file)
	}

	resp, err := upstreamGet(ctx, upstreamURL)
	if err != nil {
		return &proxyError{status: http.StatusInternalServerError, body: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return readProxyError(resp)
	}

	forwardUpstreamResponse(ctx, resp)
	return nil
}

func serveURLProxyImmutable(ctx *context.Context, upstreamURL, modulePath, version, file string) *proxyError {
	if version == "latest" {
		return &proxyError{status: http.StatusNotFound, body: "latest must be requested through @latest"}
	}

	cacheKey := goProxyCacheKey(modulePath, version, file)
	if obj, err := storage.Packages.Open(cacheKey); err == nil {
		serveCachedGoProxyObject(ctx, obj, version+"."+file, goProxyContentType(file))
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return &proxyError{status: http.StatusInternalServerError, body: err.Error()}
	}

	var result *proxyError
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
			result = readProxyError(resp)
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
		return result
	}
	if err != nil {
		return &proxyError{status: http.StatusInternalServerError, body: err.Error()}
	}

	obj, err := storage.Packages.Open(cacheKey)
	if err != nil {
		return &proxyError{status: http.StatusInternalServerError, body: err.Error()}
	}
	serveCachedGoProxyObject(ctx, obj, version+"."+file, goProxyContentType(file))
	return nil
}

func serveDirectModule(ctx *context.Context, modulePath, version, file string) *proxyError {
	repo, cleanup, err := goproxy_service.NewDirectRepository(ctx, modulePath)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return &proxyError{status: http.StatusNotFound, body: err.Error()}
		}
		return &proxyError{status: http.StatusInternalServerError, body: err.Error()}
	}
	defer cleanup()

	return serveRepositoryModule(ctx, repo, version, file)
}

func readProxyError(resp *http.Response) *proxyError {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	return &proxyError{status: resp.StatusCode, body: string(data)}
}
