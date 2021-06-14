// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package httpcache

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// GetCacheControl returns a suitable "Cache-Control" header value
func GetCacheControl() string {
	if !setting.IsProd() {
		return "no-store"
	}
	return "private, max-age=" + strconv.FormatInt(int64(setting.StaticCacheTime.Seconds()), 10)
}

// generateETag generates an ETag based on size, filename and file modification time
func generateETag(fi os.FileInfo) string {
	etag := fmt.Sprint(fi.Size()) + fi.Name() + fi.ModTime().UTC().Format(http.TimeFormat)
	return `"` + base64.StdEncoding.EncodeToString([]byte(etag)) + `"`
}

// HandleTimeCache handles time-based caching for a HTTP request
func HandleTimeCache(req *http.Request, w http.ResponseWriter, fi os.FileInfo) (handled bool) {
	w.Header().Set("Cache-Control", GetCacheControl())

	ifModifiedSince := req.Header.Get("If-Modified-Since")
	if ifModifiedSince != "" {
		t, err := time.Parse(http.TimeFormat, ifModifiedSince)
		if err == nil && fi.ModTime().Unix() <= t.Unix() {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}

	w.Header().Set("Last-Modified", fi.ModTime().Format(http.TimeFormat))
	return false
}

// HandleFileETagCache handles ETag-based caching for a HTTP request
func HandleFileETagCache(req *http.Request, w http.ResponseWriter, fi os.FileInfo) (handled bool) {
	etag := generateETag(fi)
	return HandleGenericETagCache(req, w, etag)
}

// HandleGenericETagCache handles ETag-based caching for a HTTP request.
// It returns true if the request was handled.
func HandleGenericETagCache(req *http.Request, w http.ResponseWriter, etag string) (handled bool) {
	if len(etag) > 0 {
		w.Header().Set("Etag", etag)
		if checkIfNoneMatchIsValid(req, etag) {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	w.Header().Set("Cache-Control", GetCacheControl())
	return false
}

// checkIfNoneMatchIsValid tests if the header If-None-Match matches the ETag
func checkIfNoneMatchIsValid(req *http.Request, etag string) bool {
	ifNoneMatch := req.Header.Get("If-None-Match")
	if len(ifNoneMatch) > 0 {
		for _, item := range strings.Split(ifNoneMatch, ",") {
			item = strings.TrimSpace(item)
			if item == etag {
				return true
			}
		}
	}
	return false
}
