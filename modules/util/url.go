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
	if !strings.Contains(s, "://") {
		return s, nil // paths and SCP syntax carry no credentials
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	RemoveURLCredentials(u)
	return u.String(), nil
}

// RemoveURLCredentials keeps an ssh username, it is needed to connect and isn't a secret
func RemoveURLCredentials(u *url.URL) {
	if u.Scheme == "ssh" && u.User != nil {
		u.User = url.User(u.User.Username())
	} else {
		u.User = nil
	}
}
