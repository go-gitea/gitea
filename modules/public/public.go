// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"encoding/base64"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/macaron"
)

//go:generate go run -mod=vendor main.go

// Options represents the available options to configure the macaron handler.
type Options struct {
	Directory   string
	IndexFile   string
	SkipLogging bool
	// if set to true, will enable caching. Expires header will also be set to
	// expire after the defined time.
	ExpiresAfter time.Duration
	FileSystem   http.FileSystem
	Prefix       string
}

// Custom implements the macaron static handler for serving custom assets.
func Custom(opts *Options) macaron.Handler {
	return opts.staticHandler(path.Join(setting.CustomPath, "public"))
}

// staticFileSystem implements http.FileSystem interface.
type staticFileSystem struct {
	dir *http.Dir
}

func newStaticFileSystem(directory string) staticFileSystem {
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(macaron.Root, directory)
	}
	dir := http.Dir(directory)
	return staticFileSystem{&dir}
}

func (fs staticFileSystem) Open(name string) (http.File, error) {
	return fs.dir.Open(name)
}

// StaticHandler sets up a new middleware for serving static files in the
func StaticHandler(dir string, opts *Options) macaron.Handler {
	return opts.staticHandler(dir)
}

func (opts *Options) staticHandler(dir string) macaron.Handler {
	// Defaults
	if len(opts.IndexFile) == 0 {
		opts.IndexFile = "index.html"
	}
	// Normalize the prefix if provided
	if opts.Prefix != "" {
		// Ensure we have a leading '/'
		if opts.Prefix[0] != '/' {
			opts.Prefix = "/" + opts.Prefix
		}
		// Remove any trailing '/'
		opts.Prefix = strings.TrimRight(opts.Prefix, "/")
	}
	if opts.FileSystem == nil {
		opts.FileSystem = newStaticFileSystem(dir)
	}

	return func(ctx *macaron.Context, log *log.Logger) {
		opts.handle(ctx, log, opts)
	}
}

func (opts *Options) handle(ctx *macaron.Context, log *log.Logger, opt *Options) bool {
	if ctx.Req.Method != "GET" && ctx.Req.Method != "HEAD" {
		return false
	}

	file := ctx.Req.URL.Path
	// if we have a prefix, filter requests by stripping the prefix
	if opt.Prefix != "" {
		if !strings.HasPrefix(file, opt.Prefix) {
			return false
		}
		file = file[len(opt.Prefix):]
		if file != "" && file[0] != '/' {
			return false
		}
	}

	f, err := opt.FileSystem.Open(file)
	if err != nil {
		return false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		log.Printf("[Static] %q exists, but fails to open: %v", file, err)
		return true
	}

	// Try to serve index file
	if fi.IsDir() {
		// Redirect if missing trailing slash.
		if !strings.HasSuffix(ctx.Req.URL.Path, "/") {
			http.Redirect(ctx.Resp, ctx.Req.Request, path.Clean(ctx.Req.URL.Path+"/"), http.StatusFound)
			return true
		}

		f, err = opt.FileSystem.Open(file)
		if err != nil {
			return false // Discard error.
		}
		defer f.Close()

		fi, err = f.Stat()
		if err != nil || fi.IsDir() {
			return true
		}
	}

	if !opt.SkipLogging {
		log.Println("[Static] Serving " + file)
	}

	// Add an Expires header to the static content
	if opt.ExpiresAfter > 0 {
		ctx.Resp.Header().Set("Expires", time.Now().Add(opt.ExpiresAfter).UTC().Format(http.TimeFormat))
		tag := GenerateETag(string(fi.Size()), fi.Name(), fi.ModTime().UTC().Format(http.TimeFormat))
		ctx.Resp.Header().Set("ETag", tag)
		if ctx.Req.Header.Get("If-None-Match") == tag {
			ctx.Resp.WriteHeader(304)
			return false
		}
	}

	http.ServeContent(ctx.Resp, ctx.Req.Request, file, fi.ModTime(), f)
	return true
}

// GenerateETag generates an ETag based on size, filename and file modification time
func GenerateETag(fileSize, fileName, modTime string) string {
	etag := fileSize + fileName + modTime
	return base64.StdEncoding.EncodeToString([]byte(etag))
}
