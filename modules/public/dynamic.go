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

// ServeContent serve http content
func ServeContent(w http.ResponseWriter, req *http.Request, fi os.FileInfo, modtime time.Time, content io.ReadSeeker) {
	http.ServeContent(w, req, fi.Name(), modtime, content)
}

func fileSystem(dir string) http.FileSystem {
	return http.Dir(dir)
}
