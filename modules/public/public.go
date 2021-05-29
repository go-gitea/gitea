// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Options represents the available options to configure the handler.
type Options struct {
	Directory string
	Prefix    string
}

// AssetsHandler implements the static handler for serving custom or original assets.
func AssetsHandler(opts *Options) func(resp http.ResponseWriter, req *http.Request) {
	var custPath = filepath.Join(setting.CustomPath, "public")
	if !filepath.IsAbs(custPath) {
		custPath = filepath.Join(setting.AppWorkPath, custPath)
	}

	if !filepath.IsAbs(opts.Directory) {
		opts.Directory = filepath.Join(setting.AppWorkPath, opts.Directory)
	}

	return func(resp http.ResponseWriter, req *http.Request) {
		// custom files
		if opts.handle(resp, req, http.Dir(custPath), opts.Prefix) {
			return
		}

		// internal files
		if opts.handle(resp, req, fileSystem(opts.Directory), opts.Prefix) {
			return
		}

		resp.WriteHeader(404)
	}
}

// parseAcceptEncoding parse Accept-Encoding: deflate, gzip;q=1.0, *;q=0.5 as compress methods
func parseAcceptEncoding(val string) map[string]bool {
	parts := strings.Split(val, ";")
	var types = make(map[string]bool)
	for _, v := range strings.Split(parts[0], ",") {
		types[strings.TrimSpace(v)] = true
	}
	return types
}

func (opts *Options) handle(w http.ResponseWriter, req *http.Request, fs http.FileSystem, prefix string) bool {
	if req.Method != "GET" && req.Method != "HEAD" {
		return false
	}

	file := req.URL.Path
	// if we have a prefix, filter requests by stripping the prefix
	if prefix != "" {
		if !strings.HasPrefix(file, prefix) {
			return false
		}
		file = file[len(prefix):]
		if file != "" && file[0] != '/' {
			return false
		}
	}

	f, err := fs.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		w.WriteHeader(500)
		log.Error("[Static] Open %q failed: %v", file, err)
		return true
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		w.WriteHeader(500)
		log.Error("[Static] %q exists, but fails to open: %v", file, err)
		return true
	}

	// Try to serve index file
	if fi.IsDir() {
		w.WriteHeader(404)
		return true
	}

	if httpcache.HandleFileETagCache(req, w, fi) {
		return true
	}

	serveContent(w, req, fi, fi.ModTime(), f)
	return true
}
