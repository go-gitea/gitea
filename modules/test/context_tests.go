// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package test

import (
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/context"

	"github.com/stretchr/testify/assert"
	macaron "gopkg.in/macaron.v1"
)

// MockContext mock context for unit tests
func MockContext(t *testing.T) *context.Context {
	var macaronContext *macaron.Context
	mac := macaron.New()
	mac.Get("*/", func(ctx *macaron.Context) {
		macaronContext = ctx
	})
	req, err := http.NewRequest("GET", "star", nil)
	assert.NoError(t, err)
	req.Form = url.Values{}
	mac.ServeHTTP(&mockResponseWriter{}, req)
	assert.NotNil(t, macaronContext)
	assert.EqualValues(t, req, macaronContext.Req.Request)
	macaronContext.Locale = &mockLocale{}
	macaronContext.Resp = &mockResponseWriter{}
	macaronContext.Render = &mockRender{ResponseWriter: macaronContext.Resp}
	return &context.Context{
		Context: macaronContext,
	}
}

type mockLocale struct{}

func (l mockLocale) Language() string {
	return "en"
}

func (l mockLocale) Tr(s string, _ ...interface{}) string {
	return "test translation"
}

type mockResponseWriter struct {
	status int
	size   int
}

func (rw *mockResponseWriter) Header() http.Header {
	return map[string][]string{}
}

func (rw *mockResponseWriter) Write(b []byte) (int, error) {
	rw.size += len(b)
	return len(b), nil
}

func (rw *mockResponseWriter) WriteHeader(status int) {
	rw.status = status
}

func (rw *mockResponseWriter) Flush() {
}

func (rw *mockResponseWriter) Status() int {
	return rw.status
}

func (rw *mockResponseWriter) Written() bool {
	return rw.status > 0
}

func (rw *mockResponseWriter) Size() int {
	return rw.size
}

func (rw *mockResponseWriter) Before(b macaron.BeforeFunc) {
	b(rw)
}

type mockRender struct {
	http.ResponseWriter
}

func (tr *mockRender) SetResponseWriter(rw http.ResponseWriter) {
	tr.ResponseWriter = rw
}

func (tr *mockRender) JSON(int, interface{}) {
}

func (tr *mockRender) JSONString(interface{}) (string, error) {
	return "", nil
}

func (tr *mockRender) RawData(status int, _ []byte) {
	tr.Status(status)
}

func (tr *mockRender) PlainText(status int, _ []byte) {
	tr.Status(status)
}

func (tr *mockRender) HTML(status int, _ string, _ interface{}, _ ...macaron.HTMLOptions) {
	tr.Status(status)
}

func (tr *mockRender) HTMLSet(status int, _ string, _ string, _ interface{}, _ ...macaron.HTMLOptions) {
	tr.Status(status)
}

func (tr *mockRender) HTMLSetString(string, string, interface{}, ...macaron.HTMLOptions) (string, error) {
	return "", nil
}

func (tr *mockRender) HTMLString(string, interface{}, ...macaron.HTMLOptions) (string, error) {
	return "", nil
}

func (tr *mockRender) HTMLSetBytes(string, string, interface{}, ...macaron.HTMLOptions) ([]byte, error) {
	return nil, nil
}

func (tr *mockRender) HTMLBytes(string, interface{}, ...macaron.HTMLOptions) ([]byte, error) {
	return nil, nil
}

func (tr *mockRender) XML(status int, _ interface{}) {
	tr.Status(status)
}

func (tr *mockRender) Error(status int, _ ...string) {
	tr.Status(status)
}

func (tr *mockRender) Status(status int) {
	tr.ResponseWriter.WriteHeader(status)
}

func (tr *mockRender) SetTemplatePath(string, string) {
}

func (tr *mockRender) HasTemplateSet(string) bool {
	return true
}
