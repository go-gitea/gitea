// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const viteDevPortFile = "public/assets/.vite/dev-port"

var viteDevProxy atomic.Pointer[httputil.ReverseProxy]

func getViteDevProxy() *httputil.ReverseProxy {
	if proxy := viteDevProxy.Load(); proxy != nil {
		return proxy
	}

	portFile := filepath.Join(setting.StaticRootPath, viteDevPortFile)
	data, err := os.ReadFile(portFile)
	if err != nil {
		return nil
	}
	port := strings.TrimSpace(string(data))
	if port == "" {
		return nil
	}

	target, err := url.Parse("http://localhost:" + port)
	if err != nil {
		log.Error("Failed to parse Vite dev server URL: %v", err)
		return nil
	}

	log.Info("Proxying Vite dev server requests to %s", target)
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host
		},
	}
	viteDevProxy.Store(proxy)
	return proxy
}

// ViteDevMiddleware proxies matching requests to the Vite dev server.
// It is registered as middleware in non-production mode and lazily discovers
// the Vite dev server port from the port file written by the viteDevServerPortPlugin.
func ViteDevMiddleware(resp http.ResponseWriter, req *http.Request) {
	if !IsViteDevRequest(req) {
		return
	}
	proxy := getViteDevProxy()
	if proxy == nil {
		return
	}
	proxy.ServeHTTP(resp, req)
}

// IsViteDevMode returns true if the Vite dev server port file exists.
// In production mode, the result is cached after the first check.
func IsViteDevMode() bool {
	if setting.IsProd {
		return false
	}
	portFile := filepath.Join(setting.StaticRootPath, viteDevPortFile)
	_, err := os.Stat(portFile)
	return err == nil
}

func viteDevSourceURL(name string) string {
	if strings.HasPrefix(name, "css/theme-") {
		// Only redirect built-in themes to Vite source; custom themes are served from custom/public/assets/css/
		themeFile := strings.TrimPrefix(name, "css/")
		srcPath := filepath.Join(setting.StaticRootPath, "web_src/css/themes", themeFile)
		if _, err := os.Stat(srcPath); err == nil {
			return setting.AppSubURL + "/web_src/css/themes/" + themeFile
		}
		return ""
	}
	if strings.HasPrefix(name, "css/") {
		return setting.AppSubURL + "/web_src/" + name
	}
	if name == "js/eventsource.sharedworker.js" {
		return setting.AppSubURL + "/web_src/js/features/eventsource.sharedworker.ts"
	}
	if name == "js/iife.js" {
		return setting.AppSubURL + "/__vite_iife.js"
	}
	if name == "js/index.js" {
		return setting.AppSubURL + "/web_src/js/index.ts"
	}
	return ""
}

// IsViteDevRequest returns true if the request should be proxied to the Vite dev server.
// Vite internal prefixes are defined in the Vite source:
//   - packages/vite/src/node/constants.ts (/@vite/, /@fs/, /__vite)
//   - packages/vite/src/shared/constants.ts (/@id/)
//   - packages/vite/src/node/server/ws.ts (vite-hmr, vite-ping WebSocket protocols)
//   - packages/vite/src/node/utils.ts (?import, ?raw query params)
func IsViteDevRequest(req *http.Request) bool {
	wsProtocol := req.Header.Get("Sec-WebSocket-Protocol")
	if req.Header.Get("Upgrade") == "websocket" && (wsProtocol == "vite-hmr" || wsProtocol == "vite-ping") {
		return true
	}
	path := req.URL.Path
	if strings.HasPrefix(path, "/@vite/") ||
		strings.HasPrefix(path, "/@fs/") ||
		strings.HasPrefix(path, "/@id/") ||
		strings.HasPrefix(path, "/__vite") ||
		strings.HasPrefix(path, "/node_modules/") ||
		strings.HasPrefix(path, "/web_src/") {
		return true
	}
	query := req.URL.Query()
	if _, ok := query["import"]; ok {
		return true
	}
	if _, ok := query["raw"]; ok {
		return true
	}
	return false
}
