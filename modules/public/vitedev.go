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
	if !isViteDevRequest(req) {
		return
	}
	proxy := getViteDevProxy()
	if proxy == nil {
		return
	}
	proxy.ServeHTTP(resp, req)
}

// isViteDevMode returns true if the Vite dev server port file exists.
// In production mode, the result is cached after the first check.
func isViteDevMode() bool {
	if setting.IsProd {
		return false
	}
	portFile := filepath.Join(setting.StaticRootPath, viteDevPortFile)
	_, err := os.Stat(portFile)
	return err == nil
}

func viteDevSourceURL(name string) string {
	if !isViteDevMode() {
		return ""
	}
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

// isViteDevRequest returns true if the request should be proxied to the Vite dev server.
// Ref: Vite source packages/vite/src/node/constants.ts and packages/vite/src/shared/constants.ts
func isViteDevRequest(req *http.Request) bool {
	if req.Header.Get("Upgrade") == "websocket" {
		wsProtocol := req.Header.Get("Sec-WebSocket-Protocol")
		return wsProtocol == "vite-hmr" || wsProtocol == "vite-ping"
	}
	path := req.URL.Path
	if strings.HasPrefix(path, "/@vite/") || // HMR client
		strings.HasPrefix(path, "/@fs/") || // out-of-root file access
		strings.HasPrefix(path, "/@id/") || // virtual modules
		strings.HasPrefix(path, "/__vite") || // ping endpoint, iife
		strings.HasPrefix(path, "/node_modules/") || // optimized deps
		strings.HasPrefix(path, "/web_src/") { // source files
		return true
	}
	// Vite adds ?import to non-JS/CSS asset imports:
	// - /public/assets/... (e.g. SVG icons from public/assets/img/svg/)
	// - /assets/... (e.g. assets/emoji.json)
	if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/public/assets/") {
		if _, ok := req.URL.Query()["import"]; ok {
			return true
		}
	}
	return false
}
