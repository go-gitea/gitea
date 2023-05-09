// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"bytes"
	"net/http"
)

type MockResponseWriter struct {
	header http.Header

	StatusCode int
	BodyBuffer bytes.Buffer
}

func (m *MockResponseWriter) Header() http.Header {
	return m.header
}

func (m *MockResponseWriter) Write(bytes []byte) (int, error) {
	if m.StatusCode == 0 {
		m.StatusCode = http.StatusOK
	}
	return m.BodyBuffer.Write(bytes)
}

func (m *MockResponseWriter) WriteHeader(statusCode int) {
	m.StatusCode = statusCode
}

func NewMockResponseWriter() *MockResponseWriter {
	return &MockResponseWriter{header: http.Header{}}
}
