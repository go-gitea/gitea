// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httpcache

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// SetCacheControlInHeader sets suitable cache-control headers in the response
func SetCacheControlInHeader(h http.Header, maxAge time.Duration, additionalDirectives ...string) {
	directives := make([]string, 0, 2+len(additionalDirectives))

	// "max-age=0 + must-revalidate" (aka "no-cache") is preferred instead of "no-store"
	// because browsers may restore some input fields after navigate-back / reload a page.
	if setting.IsProd {
		if maxAge == 0 {
			directives = append(directives, "max-age=0", "private", "must-revalidate")
		} else {
			directives = append(directives, "private", "max-age="+strconv.Itoa(int(maxAge.Seconds())))
		}
	} else {
		directives = append(directives, "max-age=0", "private", "must-revalidate")

		// to remind users they are using non-prod setting.
		h.Set("X-Gitea-Debug", "RUN_MODE="+setting.RunMode)
	}

	h.Set("Cache-Control", strings.Join(append(directives, additionalDirectives...), ", "))
}

func ServeContentWithCacheControl(w http.ResponseWriter, req *http.Request, name string, modTime time.Time, content io.ReadSeeker) {
	SetCacheControlInHeader(w.Header(), setting.StaticCacheTime)
	http.ServeContent(w, req, name, modTime, content)
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
	SetCacheControlInHeader(w.Header(), setting.StaticCacheTime)
	return false
}

// checkIfNoneMatchIsValid tests if the header If-None-Match matches the ETag
func checkIfNoneMatchIsValid(req *http.Request, etag string) bool {
	ifNoneMatch := req.Header.Get("If-None-Match")
	if len(ifNoneMatch) > 0 {
		for _, item := range strings.Split(ifNoneMatch, ",") {
			item = strings.TrimPrefix(strings.TrimSpace(item), "W/") // https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/ETag#directives
			if item == etag {
				return true
			}
		}
	}
	return false
}

// HandleGenericETagTimeCache handles ETag-based caching with Last-Modified caching for a HTTP request.
// It returns true if the request was handled.
func HandleGenericETagTimeCache(req *http.Request, w http.ResponseWriter, etag string, lastModified *time.Time) (handled bool) {
	if len(etag) > 0 {
		w.Header().Set("Etag", etag)
	}
	if lastModified != nil && !lastModified.IsZero() {
		// http.TimeFormat required a UTC time, refer to https://pkg.go.dev/net/http#TimeFormat
		w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	}

	if len(etag) > 0 {
		if checkIfNoneMatchIsValid(req, etag) {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	if lastModified != nil && !lastModified.IsZero() {
		ifModifiedSince := req.Header.Get("If-Modified-Since")
		if ifModifiedSince != "" {
			t, err := time.Parse(http.TimeFormat, ifModifiedSince)
			if err == nil && lastModified.Unix() <= t.Unix() {
				w.WriteHeader(http.StatusNotModified)
				return true
			}
		}
	}
	SetCacheControlInHeader(w.Header(), setting.StaticCacheTime)
	return false
}
