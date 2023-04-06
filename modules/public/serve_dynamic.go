// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !bindata

package public

import (
	"io"
	"net/http"
	"os"
	"time"
)

func fileSystem(dir string) http.FileSystem {
	return http.Dir(dir)
}

// serveContent serve http content
func serveContent(w http.ResponseWriter, req *http.Request, fi os.FileInfo, modtime time.Time, content io.ReadSeeker) {
	http.ServeContent(w, req, fi.Name(), modtime, content)
}
