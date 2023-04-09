// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/assetfs"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// Options represents the available options to configure the handler.
type Options struct {
	CorsHandler func(http.Handler) http.Handler
}

// AssetsURLPathPrefix is the path prefix for static asset files
const AssetsURLPathPrefix = "/assets/"

func CustomAssets() *assetfs.Layer {
	return assetfs.Local("custom", setting.CustomPath, "public")
}

func StaticAssets() *assetfs.Layer {
	return assetfs.Local("static", setting.StaticRootPath, "public")
}

func AssetFS() *assetfs.LayeredFS {
	if setting.HasBuiltinBindata {
		return assetfs.Layered(CustomAssets(), StaticAssets(), BuiltinAssets()) // old behavior: always include StaticAssets
	} else {
		return assetfs.Layered(CustomAssets(), BuiltinAssets()) // now BuiltinAssets is StaticAssets
	}
}

// AssetsHandlerFunc implements the static handler for serving custom or original assets.
func AssetsHandlerFunc(opts *Options) http.HandlerFunc {
	assetFS := AssetFS()

	return func(resp http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if !strings.HasPrefix(path, AssetsURLPathPrefix) {
			return
		}
		path = strings.TrimPrefix(path, AssetsURLPathPrefix)

		if req.Method != "GET" && req.Method != "HEAD" {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		if opts.CorsHandler != nil {
			var corsSent bool
			opts.CorsHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				corsSent = true
			})).ServeHTTP(resp, req)
			// If CORS is not sent, the response must have been written by other handlers
			if !corsSent {
				return
			}
		}

		if opts.handle(resp, req, assetFS, path) {
			return
		}

		resp.WriteHeader(http.StatusNotFound)
	}
}

// parseAcceptEncoding parse Accept-Encoding: deflate, gzip;q=1.0, *;q=0.5 as compress methods
func parseAcceptEncoding(val string) container.Set[string] {
	parts := strings.Split(val, ";")
	types := make(container.Set[string])
	for _, v := range strings.Split(parts[0], ",") {
		types.Add(strings.TrimSpace(v))
	}
	return types
}

// setWellKnownContentType will set the Content-Type if the file is a well-known type.
// See the comments of detectWellKnownMimeType
func setWellKnownContentType(w http.ResponseWriter, file string) {
	mimeType := detectWellKnownMimeType(path.Ext(file))
	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}
}

func (opts *Options) handle(w http.ResponseWriter, req *http.Request, fs http.FileSystem, file string) bool {
	// actually, fs (http.FileSystem) is designed to be a safe interface, relative paths won't bypass its parent directory, it's also fine to do a clean here
	f, err := fs.Open(util.PathJoinRelX(file))
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Error("[Static] Open %q failed: %v", file, err)
		return true
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error("[Static] %q exists, but fails to open: %v", file, err)
		return true
	}

	// Try to serve index file
	if fi.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		return true
	}

	if httpcache.HandleFileETagCache(req, w, fi) {
		return true
	}

	serveContent(w, req, fi, fi.ModTime(), f)
	return true
}

type GzipBytesProvider interface {
	GzipBytes() []byte
}

// serveContent serve http content
func serveContent(w http.ResponseWriter, req *http.Request, fi os.FileInfo, modtime time.Time, content io.ReadSeeker) {
	setWellKnownContentType(w, fi.Name())

	encodings := parseAcceptEncoding(req.Header.Get("Accept-Encoding"))
	if encodings.Contains("gzip") {
		// try to provide gzip content directly from bindata (provided by vfsgen۰CompressedFileInfo)
		if compressed, ok := fi.(GzipBytesProvider); ok {
			rdGzip := bytes.NewReader(compressed.GzipBytes())
			// all gzipped static files (from bindata) are managed by Gitea, so we can make sure every file has the correct ext name
			// then we can get the correct Content-Type, we do not need to do http.DetectContentType on the decompressed data
			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", "application/octet-stream")
			}
			w.Header().Set("Content-Encoding", "gzip")
			http.ServeContent(w, req, fi.Name(), modtime, rdGzip)
			return
		}
	}

	http.ServeContent(w, req, fi.Name(), modtime, content)
	return
}
