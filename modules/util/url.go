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

// URLJoin joins url components, like path.Join, but preserving contents
func URLJoin(base string, elems ...string) string {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}

	var fragment string
	last := len(elems) - 1
	if len(elems) > 0 {
		if strings.Contains(elems[last], "#") {
			elems[last], fragment, _ = strings.Cut(elems[last], "#")
		}
		elems[last] = strings.TrimSuffix(elems[last], "/") // keep old behaviour
	}

	joinedURL := baseURL.JoinPath(elems...)
	joinedURL.Fragment = fragment

	if !baseURL.IsAbs() && !strings.HasPrefix(base, "/") {
		return strings.TrimPrefix(joinedURL.String(), "/") // Removing leading '/' if needed
	}
	return joinedURL.String()
}

func SanitizeURL(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	u.User = nil
	return u.String(), nil
}
