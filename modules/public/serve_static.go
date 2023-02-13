// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !servedynamic

package public

import (
	"io"
	"net/http"
	"os"
	"time"

	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/public"
)

func Asset(name string) ([]byte, error) {
	return public.Asset(name)
}

func AssetNames() []string {
	return public.AssetNames()
}

// GlobalModTime provide a global mod time for embedded asset files
func GlobalModTime(filename string) time.Time {
	return timeutil.GetExecutableModTime()
}

func fileSystem(dir string) http.FileSystem {
	return http.FS(&public.PublicFS)
}

func AssetIsDir(name string) (bool, error) {
	if f, err := public.PublicFS.Open(name); err != nil {
		return false, err
	} else {
		defer f.Close()
		if fi, err := f.Stat(); err != nil {
			return false, err
		} else {
			return fi.IsDir(), nil
		}
	}
}

// serveContent serve http content
func serveContent(w http.ResponseWriter, req *http.Request, fi os.FileInfo, modtime time.Time, content io.ReadSeeker) {
	http.ServeContent(w, req, fi.Name(), modtime, content)
	return
}
