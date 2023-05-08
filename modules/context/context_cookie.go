// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"

	"github.com/minio/sha256-simd"
	"golang.org/x/crypto/pbkdf2"
)

const CookieNameFlash = "gitea_flash"

func removeSessionCookieHeader(w http.ResponseWriter) {
	cookies := w.Header()["Set-Cookie"]
	w.Header().Del("Set-Cookie")
	for _, cookie := range cookies {
		if strings.HasPrefix(cookie, setting.SessionConfig.CookieName+"=") {
			continue
		}
		w.Header().Add("Set-Cookie", cookie)
	}
}

// SetSiteCookie convenience function to set most cookies consistently
// CSRF and a few others are the exception here
func (ctx *Context) SetSiteCookie(name, value string, maxAge int) {
	middleware.SetSiteCookie(ctx.Resp, name, value, maxAge)
}

// DeleteSiteCookie convenience function to delete most cookies consistently
// CSRF and a few others are the exception here
func (ctx *Context) DeleteSiteCookie(name string) {
	middleware.SetSiteCookie(ctx.Resp, name, "", -1)
}

// GetSiteCookie returns given cookie value from request header.
func (ctx *Context) GetSiteCookie(name string) string {
	return middleware.GetSiteCookie(ctx.Req, name)
}

// GetSuperSecureCookie returns given cookie value from request header with secret string.
func (ctx *Context) GetSuperSecureCookie(secret, name string) (string, bool) {
	val := ctx.GetSiteCookie(name)
	return ctx.CookieDecrypt(secret, val)
}

// CookieDecrypt returns given value from with secret string.
func (ctx *Context) CookieDecrypt(secret, val string) (string, bool) {
	if val == "" {
		return "", false
	}

	text, err := hex.DecodeString(val)
	if err != nil {
		return "", false
	}

	key := pbkdf2.Key([]byte(secret), []byte(secret), 1000, 16, sha256.New)
	text, err = util.AESGCMDecrypt(key, text)
	return string(text), err == nil
}

// SetSuperSecureCookie sets given cookie value to response header with secret string.
func (ctx *Context) SetSuperSecureCookie(secret, name, value string, maxAge int) {
	text := ctx.CookieEncrypt(secret, value)
	ctx.SetSiteCookie(name, text, maxAge)
}

// CookieEncrypt encrypts a given value using the provided secret
func (ctx *Context) CookieEncrypt(secret, value string) string {
	key := pbkdf2.Key([]byte(secret), []byte(secret), 1000, 16, sha256.New)
	text, err := util.AESGCMEncrypt(key, []byte(value))
	if err != nil {
		panic("error encrypting cookie: " + err.Error())
	}

	return hex.EncodeToString(text)
}

// GetCookieInt returns cookie result in int type.
func (ctx *Context) GetCookieInt(name string) int {
	r, _ := strconv.Atoi(ctx.GetSiteCookie(name))
	return r
}

// GetCookieInt64 returns cookie result in int64 type.
func (ctx *Context) GetCookieInt64(name string) int64 {
	r, _ := strconv.ParseInt(ctx.GetSiteCookie(name), 10, 64)
	return r
}

// GetCookieFloat64 returns cookie result in float64 type.
func (ctx *Context) GetCookieFloat64(name string) float64 {
	v, _ := strconv.ParseFloat(ctx.GetSiteCookie(name), 64)
	return v
}
