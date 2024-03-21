// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

func urlIsRelative(s string, u *url.URL) bool {
	// Unfortunately browsers consider a redirect Location with preceding "//", "\\", "/\" and "\/" as meaning redirect to "http(s)://REST_OF_PATH"
	// Therefore we should ignore these redirect locations to prevent open redirects
	if len(s) > 1 && (s[0] == '/' || s[0] == '\\') && (s[1] == '/' || s[1] == '\\') {
		return false
	}
	return u != nil && u.Scheme == "" && u.Host == ""
}

// IsRelativeURL detects if a URL is relative (no scheme or host)
func IsRelativeURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && urlIsRelative(s, u)
}

func IsCurrentGiteaSiteURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	if u.Path != "" {
		u.Path = "/" + util.PathJoinRelX(u.Path)
		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
		}
	}
	if urlIsRelative(s, u) {
		return u.Path == "" || strings.HasPrefix(strings.ToLower(u.Path), strings.ToLower(setting.AppSubURL+"/"))
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return strings.HasPrefix(strings.ToLower(u.String()), strings.ToLower(setting.AppURL))
}
