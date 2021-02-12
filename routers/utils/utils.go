// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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

// IsValidSlackChannel validates a channel name conforms to what slack expects.
// It makes sure a channel name cannot be empty and invalid ( only an # )
func IsValidSlackChannel(channelName string) bool {
	switch len(strings.TrimSpace(channelName)) {
	case 0:
		return false
	case 1:
		// Keep default behaviour where a channel name is still
		// valid without an #
		// But if it contains only an #, it should be regarded as
		// invalid
		if channelName[0] == '#' {
			return false
		}
	}

	return true
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
