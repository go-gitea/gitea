// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type RequestContextKeyStruct struct{}

var RequestContextKey = RequestContextKeyStruct{}

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

func getRequestScheme(req *http.Request) string {
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-Proto
	if s := req.Header.Get("X-Forwarded-Proto"); s != "" {
		return s
	}
	if s := req.Header.Get("X-Forwarded-Protocol"); s != "" {
		return s
	}
	if s := req.Header.Get("X-Url-Scheme"); s != "" {
		return s
	}
	if s := req.Header.Get("Front-End-Https"); s != "" {
		return util.Iif(s == "on", "https", "http")
	}
	if s := req.Header.Get("X-Forwarded-Ssl"); s != "" {
		return util.Iif(s == "on", "https", "http")
	}
	return ""
}

func getForwardedHost(req *http.Request) string {
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-Host
	return req.Header.Get("X-Forwarded-Host")
}

// GuessCurrentAppURL tries to guess the current full app URL (with sub-path) by http headers. It always has a '/' suffix, exactly the same as setting.AppURL
func GuessCurrentAppURL(ctx context.Context) string {
	return GuessCurrentHostURL(ctx) + setting.AppSubURL + "/"
}

// GuessCurrentHostURL tries to guess the current full host URL (no sub-path) by http headers, there is no trailing slash.
func GuessCurrentHostURL(ctx context.Context) string {
	req, ok := ctx.Value(RequestContextKey).(*http.Request)
	if !ok {
		return strings.TrimSuffix(setting.AppURL, setting.AppSubURL+"/")
	}
	// If no scheme provided by reverse proxy, then do not guess the AppURL, use the configured one.
	// At the moment, if site admin doesn't configure the proxy headers correctly, then Gitea would guess wrong.
	// There are some cases:
	// 1. The reverse proxy is configured correctly, it passes "X-Forwarded-Proto/Host" headers. Perfect, Gitea can handle it correctly.
	// 2. The reverse proxy is not configured correctly, doesn't pass "X-Forwarded-Proto/Host" headers, eg: only one "proxy_pass http://gitea:3000" in Nginx.
	// 3. There is no reverse proxy.
	// Without an extra config option, Gitea is impossible to distinguish between case 2 and case 3,
	// then case 2 would result in wrong guess like guessed AppURL becomes "http://gitea:3000/", which is not accessible by end users.
	// So in the future maybe it should introduce a new config option, to let site admin decide how to guess the AppURL.
	reqScheme := getRequestScheme(req)
	if reqScheme == "" {
		return strings.TrimSuffix(setting.AppURL, setting.AppSubURL+"/")
	}
	reqHost := getForwardedHost(req)
	if reqHost == "" {
		reqHost = req.Host
	}
	return reqScheme + "://" + reqHost
}

// MakeAbsoluteURL tries to make a link to an absolute URL:
// * If link is empty, it returns the current app URL.
// * If link is absolute, it returns the link.
// * Otherwise, it returns the current host URL + link, the link itself should have correct sub-path (AppSubURL) if needed.
func MakeAbsoluteURL(ctx context.Context, link string) string {
	if link == "" {
		return GuessCurrentAppURL(ctx)
	}
	if !IsRelativeURL(link) {
		return link
	}
	return GuessCurrentHostURL(ctx) + "/" + strings.TrimPrefix(link, "/")
}

func IsCurrentGiteaSiteURL(ctx context.Context, s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	if u.Path != "" {
		cleanedPath := util.PathJoinRelX(u.Path)
		if cleanedPath == "" || cleanedPath == "." {
			u.Path = "/"
		} else {
			u.Path += "/" + cleanedPath + "/"
		}
	}
	if urlIsRelative(s, u) {
		return u.Path == "" || strings.HasPrefix(strings.ToLower(u.Path), strings.ToLower(setting.AppSubURL+"/"))
	}
	if u.Path == "" {
		u.Path = "/"
	}
	urlLower := strings.ToLower(u.String())
	return strings.HasPrefix(urlLower, strings.ToLower(setting.AppURL)) || strings.HasPrefix(urlLower, strings.ToLower(GuessCurrentAppURL(ctx)))
}
