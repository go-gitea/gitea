// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"encoding/base64"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/util"
)

// Flash represents a one time data transfer between two requests.
type Flash struct {
	DataStore reqctx.RequestDataStore
	msgValues map[string]string

	// message is in HTML format, and is sanitized when rendering (ref: RenderFlashMessage)
	ErrorMsg, WarningMsg, InfoMsg, SuccessMsg string
}

func (f *Flash) set(name, msg string, current ...bool) {
	if f.msgValues == nil {
		f.msgValues = map[string]string{}
	}
	showInCurrentPage := len(current) > 0 && current[0]
	if showInCurrentPage {
		// assign it to the context data, then the template can use ".Flash.XxxMsg" to render the message
		f.DataStore.GetData()["Flash"] = f
	} else {
		// the message map will be saved into the cookie and be shown in next response (a new page response which decodes the cookie)
		f.msgValues[name] = msg
	}
}

var flashMessageReplacer = sync.OnceValue(func() (ret struct {
	encoder, decoder *strings.Replacer
},
) {
	ret.encoder = strings.NewReplacer("\\", "\\\\", "\n", "\\n")
	ret.decoder = strings.NewReplacer("\\\\", "\\", "\\n", "\n")
	return ret
})

func flashMessageEncode(m map[string]string, limit int) string {
	if len(m) == 0 {
		return ""
	}
	buf := strings.Builder{}
	buf.Grow(200) // most messages are short, use a good initial size
	for k, v := range m {
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(flashMessageReplacer().encoder.Replace(v))
		buf.WriteByte('\n')
	}
	s := buf.String()
	n := min(len(s), limit)
	return base64.RawURLEncoding.EncodeToString(util.UnsafeStringToBytes(s[:n]))
}

func flashMessageDecode(s string) map[string]string {
	if s == "" {
		return nil
	}

	if strings.Contains(s, "=") {
		// legacy format, just parse it as query string
		vals, err := url.ParseQuery(s)
		if err != nil {
			return nil
		}
		m := make(map[string]string, len(vals))
		for k, v := range vals {
			if len(v) > 0 {
				m[k] = v[0]
			}
		}
		return m
	}

	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	m := make(map[string]string)
	for line := range strings.SplitSeq(util.UnsafeBytesToString(data), "\n") {
		if line == "" {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		if v != "" {
			v = flashMessageReplacer().decoder.Replace(v)
			m[k] = strings.ToValidUTF8(v, "")
		}
	}
	return m
}

func (f *Flash) EncodeToString() string {
	// modern browsers support 4096 bytes for a cookie, we use base64 for encoding, so limit to 3000
	return flashMessageEncode(f.msgValues, 3000)
}

// Error sets error message
func (f *Flash) Error[T string | template.HTML](msg T, current ...bool) {
	f.ErrorMsg = string(msg)
	f.set("error", f.ErrorMsg, current...)
}

// Warning sets warning message
func (f *Flash) Warning[T string | template.HTML](msg T, current ...bool) {
	f.WarningMsg = string(msg)
	f.set("warning", f.WarningMsg, current...)
}

// Info sets info message
func (f *Flash) Info[T string | template.HTML](msg T, current ...bool) {
	f.InfoMsg = string(msg)
	f.set("info", f.InfoMsg, current...)
}

// Success sets success message
func (f *Flash) Success[T string | template.HTML](msg T, current ...bool) {
	f.SuccessMsg = string(msg)
	f.set("success", f.SuccessMsg, current...)
}

func ParseCookieFlashMessage(val string) *Flash {
	m := flashMessageDecode(val)
	if len(m) == 0 {
		return nil
	}
	return &Flash{ErrorMsg: m["error"], SuccessMsg: m["success"], InfoMsg: m["info"], WarningMsg: m["warning"]}
}

func GetSiteCookieFlashMessage(dataStore reqctx.RequestDataStore, req *http.Request, cookieName string) (string, *Flash) {
	lastFlashCookie := GetSiteCookie(req, cookieName)
	lastFlashMsg := ParseCookieFlashMessage(lastFlashCookie)
	if lastFlashMsg != nil {
		lastFlashMsg.DataStore = dataStore
		return lastFlashCookie, lastFlashMsg
	}
	return lastFlashCookie, nil
}
