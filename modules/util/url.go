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
	// SCP-like SSH short syntax (e.g. git@host:owner/repo.git) is not a valid URL,
	// so url.Parse mangles it. Detect and return as-is — there are no embedded
	// credentials in this form to sanitize.
	trimmed := strings.TrimSpace(s)
	if !strings.Contains(trimmed, "://") {
		at := strings.Index(trimmed, "@")
		colon := strings.Index(trimmed, ":")
		slash := strings.Index(trimmed, "/")
		if at > 0 && colon > at && (slash < 0 || slash > colon) {
			return trimmed, nil
		}
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	u.User = nil
	return u.String(), nil
}
