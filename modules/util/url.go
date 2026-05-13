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

// StripURL strips userinfo, query, and fragment from s for safe logging
// (e.g. basic-auth userinfo, signed-URL credentials in the query string).
// Returns "<unparseable url>" on parse error.
func StripURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return "<unparseable url>"
	}
	stripped := url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}
	return stripped.String()
}
