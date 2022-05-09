// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build bindata

package public

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/timeutil"
)

// GlobalModTime provide a global mod time for embedded asset files
func GlobalModTime(filename string) time.Time {
	return timeutil.GetExecutableModTime()
}

func fileSystem(dir string) http.FileSystem {
	return Assets
}

func Asset(name string) ([]byte, error) {
	f, err := Assets.Open("/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func AssetNames() []string {
	realFS := Assets.(vfsgen۰FS)
	results := make([]string, 0, len(realFS))
	for k := range realFS {
		results = append(results, k[1:])
	}
	return results
}

func AssetIsDir(name string) (bool, error) {
	if f, err := Assets.Open("/" + name); err != nil {
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
	encodings := parseAcceptEncoding(req.Header.Get("Accept-Encoding"))
	if encodings["gzip"] {
		if cf, ok := fi.(*vfsgen۰CompressedFileInfo); ok {
			rdGzip := bytes.NewReader(cf.GzipBytes())
			// all static files are managed by Gitea, so we can make sure every file has the correct ext name
			// then we can get the correct Content-Type, we do not need to do http.DetectContentType on the decompressed data
			mimeType := detectWellKnownMimeType(filepath.Ext(fi.Name()))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Content-Encoding", "gzip")
			http.ServeContent(w, req, fi.Name(), modtime, rdGzip)
			return
		}
	}

	http.ServeContent(w, req, fi.Name(), modtime, content)
	return
}
