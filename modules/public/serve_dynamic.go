// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build servedynamic

package public

import (
	"errors"
	"io"
	"net/http"
	"os"
	"time"
)

func Asset(name string) ([]byte, error) {
	return nil, errors.New("assets are not built-in when servedynamic tag is enabled")
}

func AssetNames() []string {
	return nil
}

func fileSystem(dir string) http.FileSystem {
	return http.Dir(dir)
}

// serveContent serve http content
func serveContent(w http.ResponseWriter, req *http.Request, fi os.FileInfo, modtime time.Time, content io.ReadSeeker) {
	http.ServeContent(w, req, fi.Name(), modtime, content)
}
