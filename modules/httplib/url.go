// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

// IsRiskyRedirectURL returns true if the URL is considered risky for redirects
func IsRiskyRedirectURL(s string) bool {
	// Unfortunately browsers consider a redirect Location with preceding "//", "\\", "/\" and "\/" as meaning redirect to "http(s)://REST_OF_PATH"
	// Therefore we should ignore these redirect locations to prevent open redirects
	if len(s) > 1 && (s[0] == '/' || s[0] == '\\') && (s[1] == '/' || s[1] == '\\') {
		return true
	}

	u, err := url.Parse(s)
	if err != nil || ((u.Scheme != "" || u.Host != "") && !strings.HasPrefix(strings.ToLower(s), strings.ToLower(setting.AppURL))) {
		return true
	}

	return false
}
