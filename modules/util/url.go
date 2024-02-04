// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"net/url"
	"path"
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

// URLJoin joins url components, like path.Join, but preserving contents
func URLJoin(base string, elems ...string) string {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	joinedPath := path.Join(elems...)
	argURL, err := url.Parse(joinedPath)
	if err != nil {
		return ""
	}
	joinedURL := baseURL.ResolveReference(argURL).String()
	if !baseURL.IsAbs() && !strings.HasPrefix(base, "/") {
		return joinedURL[1:] // Removing leading '/' if needed
	}
	return joinedURL
}

func SanitizeURL(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	u.User = nil
	return u.String(), nil
}
