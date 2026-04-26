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
	"time"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/routing"
)

const viteDevPortFile = "public/assets/.vite/dev-port"

var viteDevProxy atomic.Pointer[httputil.ReverseProxy]

func getViteDevServerBaseURL() string {
	portFile := filepath.Join(setting.StaticRootPath, viteDevPortFile)
	portContent, _ := os.ReadFile(portFile)
	port := strings.TrimSpace(string(portContent))
	if port == "" {
		return ""
	}
	return "http://localhost:" + port
}

func getViteDevProxy() *httputil.ReverseProxy {
	if proxy := viteDevProxy.Load(); proxy != nil {
		return proxy
	}

	viteDevServerBaseURL := getViteDevServerBaseURL()
	if viteDevServerBaseURL == "" {
		return nil
	}

	target, err := url.Parse(viteDevServerBaseURL)
	if err != nil {
		log.Error("Failed to parse Vite dev server base URL %s, err: %v", viteDevServerBaseURL, err)
		return nil
	}

	// there is a strange error log (from Golang's HTTP package)
	// 2026/03/28 19:50:13 modules/log/misc.go:72:(*loggerToWriter).Write() [I] Unsolicited response received on idle HTTP channel starting with "HTTP/1.1 400 Bad Request\r\n\r\n"; err=<nil>
	// maybe it is caused by that the Vite dev server doesn't support keep-alive connections? or different keep-alive timeouts?
	transport := &http.Transport{
		IdleConnTimeout:       5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}
	log.Info("Proxying Vite dev server requests to %s", target)
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host
		},
		ModifyResponse: func(resp *http.Response) error {
			// add a header to indicate the Vite dev server port,
			// make developers know that this request is proxied to Vite dev server and which port it is
			resp.Header.Add("X-Gitea-Vite-Dev-Server", viteDevServerBaseURL)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if r.Context().Err() != nil {
				return // request cancelled (e.g. client disconnected), silently ignore
			}
			log.Error("Error proxying to Vite dev server: %v", err)
			http.Error(w, "Error proxying to Vite dev server: "+err.Error(), http.StatusBadGateway)
		},
	}
	viteDevProxy.Store(proxy)
	return proxy
}

// ViteDevMiddleware proxies matching requests to the Vite dev server.
// It is registered as middleware in non-production mode and lazily discovers
// the Vite dev server port from the port file written by the viteDevServerPortPlugin.
// It is needed because there are container-based development, only Gitea web server's port is exposed.
func ViteDevMiddleware(next http.Handler) http.Handler {
	markLongPolling := routing.MarkLongPolling()
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if !isViteDevRequest(req) {
			next.ServeHTTP(resp, req)
			return
		}
		proxy := getViteDevProxy()
		if proxy == nil {
			next.ServeHTTP(resp, req)
			return
		}
		markLongPolling(proxy).ServeHTTP(resp, req)
	})
}

var viteDevModeCheck atomic.Pointer[struct {
	isDev bool
	time  time.Time
}]

// IsViteDevMode returns true if the Vite dev server port file exists and the server is alive
func IsViteDevMode() bool {
	if setting.IsProd {
		return false
	}

	now := time.Now()
	lastCheck := viteDevModeCheck.Load()
	if lastCheck != nil && time.Now().Sub(lastCheck.time) < time.Second {
		return lastCheck.isDev
	}

	viteDevServerBaseURL := getViteDevServerBaseURL()
	if viteDevServerBaseURL == "" {
		return false
	}

	req := httplib.NewRequest(viteDevServerBaseURL+"/web_src/js/__vite_dev_server_check", "GET")
	resp, _ := req.Response()
	if resp != nil {
		_ = resp.Body.Close()
	}
	isDev := resp != nil && resp.StatusCode == http.StatusOK
	viteDevModeCheck.Store(&struct {
		isDev bool
		time  time.Time
	}{
		isDev: isDev,
		time:  now,
	})
	return isDev
}

func detectWebSrcPath(webSrcPath string) string {
	localPath := util.FilePathJoinAbs(setting.StaticRootPath, "web_src", webSrcPath)
	if _, err := os.Stat(localPath); err == nil {
		return setting.AppSubURL + "/web_src/" + webSrcPath
	}
	return ""
}

func viteDevSourceURL(name string) string {
	if strings.HasPrefix(name, "css/theme-") {
		// Only redirect built-in themes to Vite source; custom themes are served from custom/public/assets/css/
		themeFilePath := "css/themes/" + strings.TrimPrefix(name, "css/")
		if srcPath := detectWebSrcPath(themeFilePath); srcPath != "" {
			return srcPath
		}
	}
	// try to map ".js" files to ".ts" files
	pathPrefix, ok := strings.CutSuffix(name, ".js")
	if ok {
		if srcPath := detectWebSrcPath(pathPrefix + ".ts"); srcPath != "" {
			return srcPath
		}
	}
	// for all others that the names match
	return detectWebSrcPath(name)
}

// isViteDevRequest returns true if the request should be proxied to the Vite dev server.
// Ref: Vite source packages/vite/src/node/constants.ts and packages/vite/src/shared/constants.ts
func isViteDevRequest(req *http.Request) bool {
	if req.Header.Get("Upgrade") == "websocket" {
		wsProtocol := req.Header.Get("Sec-WebSocket-Protocol")
		return wsProtocol == "vite-hmr" || wsProtocol == "vite-ping"
	}
	path := req.URL.Path

	// vite internal requests
	if strings.HasPrefix(path, "/@vite/") /* HMR client */ ||
		strings.HasPrefix(path, "/@fs/") /* out-of-root file access, see vite.config.ts: fs.allow */ ||
		strings.HasPrefix(path, "/@id/") /* virtual modules */ {
		return true
	}

	// local source requests (VITE-DEV-SERVER-SECURITY: don't serve sensitive files outside the allowed paths)
	if strings.HasPrefix(path, "/node_modules/") ||
		strings.HasPrefix(path, "/public/assets/") ||
		strings.HasPrefix(path, "/web_src/") {
		return true
	}

	// Vite uses a path relative to project root and adds "?import" to non-JS/CSS asset imports:
	// - {WebSite}/public/assets/... (e.g. SVG icons from "{RepoRoot}/public/assets/img/svg/")
	// - {WebSite}/assets/emoji.json: it is an exception for the frontend assets, it is imported by JS code, but:
	//   - KEEP IN MIND: all static frontend assets are served from "{AssetFS}/assets" to "{WebSite}/assets" by Gitea Web Server
	//   - "{AssetFS}" is a layered filesystem from "{RepoRoot}/public" or embedded assets, and user's custom files in "{CustomPath}/public"
	//   - "{RepoRoot}/assets/emoji.json" just happens to have the dir name "assets", it is not related to frontend assets
	//   - BAD DESIGN: indeed it is a "conflicted and polluted name" sample
	if path == "/assets/emoji.json" {
		return true
	}
	return false
}
