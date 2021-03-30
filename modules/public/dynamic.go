// +build !bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"io"
	"net/http"
	"os"
	"time"
)

// Static implements the static handler for serving assets.
func Static(opts *Options) func(next http.Handler) http.Handler {
	return opts.staticHandler(opts.Directory)
}

// ServeContent serve http content
func ServeContent(w http.ResponseWriter, req *http.Request, fi os.FileInfo, modtime time.Time, content io.ReadSeeker) {
	http.ServeContent(w, req, fi.Name(), modtime, content)
}
