// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"html"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

// RemoveUsernameParameterSuffix returns the username parameter without the (fullname) suffix - leaving just the username
func RemoveUsernameParameterSuffix(name string) string {
	if index := strings.Index(name, " ("); index >= 0 {
		name = name[:index]
	}
	return name
}

// SanitizeFlashErrorString will sanitize a flash error string
func SanitizeFlashErrorString(x string) string {
	return strings.ReplaceAll(html.EscapeString(x), "\n", "<br>")
}

// IsExternalURL checks if rawURL points to an external URL like http://example.com
func IsExternalURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	appURL, _ := url.Parse(setting.AppURL)
	if len(parsed.Host) != 0 && strings.Replace(parsed.Host, "www.", "", 1) != strings.Replace(appURL.Host, "www.", "", 1) {
		return true
	}
	return false
}
