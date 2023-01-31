// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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

// AddCacheControlToHeader adds suitable cache-control headers to response
func AddCacheControlToHeader(h http.Header, maxAge time.Duration, additionalDirectives ...string) {
	directives := make([]string, 0, 2+len(additionalDirectives))

	if setting.IsProd {
		if maxAge == 0 {
			directives = append(directives, "no-store")
		} else {
			directives = append(directives, "private", "max-age="+strconv.Itoa(int(maxAge.Seconds())))
		}
	} else {
		directives = append(directives, "no-store")

		// to remind users they are using non-prod setting.
		h.Add("X-Gitea-Debug", "RUN_MODE="+setting.RunMode)
	}

	h.Set("Cache-Control", strings.Join(append(directives, additionalDirectives...), ", "))
}

// generateETag generates an ETag based on size, filename and file modification time
func generateETag(fi os.FileInfo) string {
	etag := fmt.Sprint(fi.Size()) + fi.Name() + fi.ModTime().UTC().Format(http.TimeFormat)
	return `"` + base64.StdEncoding.EncodeToString([]byte(etag)) + `"`
}

// HandleTimeCache handles time-based caching for a HTTP request
func HandleTimeCache(req *http.Request, w http.ResponseWriter, fi os.FileInfo) (handled bool) {
	return HandleGenericTimeCache(req, w, fi.ModTime())
}

// HandleGenericTimeCache handles time-based caching for a HTTP request
func HandleGenericTimeCache(req *http.Request, w http.ResponseWriter, lastModified time.Time) (handled bool) {
	AddCacheControlToHeader(w.Header(), setting.StaticCacheTime)

	ifModifiedSince := req.Header.Get("If-Modified-Since")
	if ifModifiedSince != "" {
		t, err := time.Parse(http.TimeFormat, ifModifiedSince)
		if err == nil && lastModified.Unix() <= t.Unix() {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}

	w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
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
	AddCacheControlToHeader(w.Header(), setting.StaticCacheTime)
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

// HandleGenericETagTimeCache handles ETag-based caching with Last-Modified caching for a HTTP request.
// It returns true if the request was handled.
func HandleGenericETagTimeCache(req *http.Request, w http.ResponseWriter, etag string, lastModified time.Time) (handled bool) {
	if len(etag) > 0 {
		w.Header().Set("Etag", etag)
	}
	if !lastModified.IsZero() {
		w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
	}

	if len(etag) > 0 {
		if checkIfNoneMatchIsValid(req, etag) {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	if !lastModified.IsZero() {
		ifModifiedSince := req.Header.Get("If-Modified-Since")
		if ifModifiedSince != "" {
			t, err := time.Parse(http.TimeFormat, ifModifiedSince)
			if err == nil && lastModified.Unix() <= t.Unix() {
				w.WriteHeader(http.StatusNotModified)
				return true
			}
		}
	}
	AddCacheControlToHeader(w.Header(), setting.StaticCacheTime)
	return false
}
