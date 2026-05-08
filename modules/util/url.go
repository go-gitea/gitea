// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"net/url"
	"strings"
)

// PathEscapeSegments escapes segments of a path while not escaping forward slash
func PathEscapeSegments(path string) string {
	slice := strings.Split(path, "/")
	for index := range slice {
		slice[index] = url.PathEscape(slice[index])
	}
	escapedPath := strings.Join(slice, "/")
	return escapedPath
}

func SanitizeURL(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	u.User = nil
	return u.String(), nil
}

// StripURL returns the scheme, host, and path portions of s with userinfo,
// query string, and fragment removed. Intended for logging URLs whose
// userinfo or query string may carry credentials (e.g. https://user:pass@…
// or signed S3/GCS/Cloudinary URLs whose signatures live in the query
// string). Returns "<unparseable url>" if s cannot be parsed.
//
// Unlike SanitizeURL (which only strips userinfo and is used by callers
// such as mirroring/indexing/migrations that still need the query string
// to actually use the URL), StripURL is for logging only.
func StripURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return "<unparseable url>"
	}
	stripped := url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}
	return stripped.String()
}
