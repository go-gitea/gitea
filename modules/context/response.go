// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import "net/http"

// ResponseWriter represents a response writer for HTTP
type ResponseWriter interface {
	http.ResponseWriter
	Flush()
	Status() int
}

var (
	_ ResponseWriter = &Response{}
)

// Response represents a response
type Response struct {
	http.ResponseWriter
	status int
}

// Write writes bytes to HTTP endpoint
func (r *Response) Write(bs []byte) (int, error) {
	size, err := r.ResponseWriter.Write(bs)
	if err != nil {
		return 0, err
	}
	if r.status == 0 {
		r.WriteHeader(200)
	}
	return size, nil
}

// WriteHeader write status code
func (r *Response) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Flush flush cached data
func (r *Response) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Status returned status code written
func (r *Response) Status() int {
	return r.status
}

// NewResponse creates a response
func NewResponse(resp http.ResponseWriter) *Response {
	if v, ok := resp.(*Response); ok {
		return v
	}
	return &Response{resp, 0}
}
