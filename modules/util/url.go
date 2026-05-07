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

// SanitizeURLForLog returns a redacted form of a URL safe to include in
// log lines. It strips userinfo (e.g. https://user:pass@…), the query
// string (which may contain signed-URL credentials such as AWS S3 / GCS /
// Cloudinary signatures), and the fragment, leaving only scheme+host+path.
// On a parse failure the placeholder "<unparseable url>" is returned to
// avoid leaking the raw URL into logs.
//
// Unlike SanitizeURL this is intended exclusively for logging: callers
// that still need to USE the URL (mirroring, indexing, migrations, etc.)
// should keep using SanitizeURL because they need the query string
// preserved.
func SanitizeURLForLog(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return "<unparseable url>"
	}
	redacted := url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}
	return redacted.String()
}
