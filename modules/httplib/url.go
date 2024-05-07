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

func guessRequestScheme(req *http.Request) string {
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
	if req.TLS != nil {
		return "https"
	}
	return "http"
}

func guessRequestHost(req *http.Request) string {
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-Host
	if s := req.Header.Get("X-Forwarded-Host"); s != "" {
		return s
	}
	if req.Host != "" {
		return req.Host // Golang: the Host header is promoted to the Request.Host field and removed from the Header map.
	}
	return ""
}

// GuessCurrentAppURL tries to guess the current full URL by http headers. It always has a '/' suffix, exactly the same as setting.AppURL
func GuessCurrentAppURL(ctx context.Context) string {
	req, ok := ctx.Value(RequestContextKey).(*http.Request)
	if !ok {
		return setting.AppURL
	}
	if host := guessRequestHost(req); host != "" {
		return guessRequestScheme(req) + "://" + host + setting.AppSubURL + "/"
	}
	return setting.AppURL
}

func MakeAbsoluteURL(ctx context.Context, s string) string {
	if IsRelativeURL(s) {
		return GuessCurrentAppURL(ctx) + strings.TrimPrefix(s, "/")
	}
	return s
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
