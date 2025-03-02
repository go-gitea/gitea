// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"

	web_types "code.gitea.io/gitea/modules/web/types"
)

// ResponseWriter represents a response writer for HTTP
type ResponseWriter interface {
	http.ResponseWriter              // provides Header/Write/WriteHeader
	http.Flusher                     // provides Flush
	web_types.ResponseStatusProvider // provides WrittenStatus

	Before(fn func(ResponseWriter))
	WrittenSize() int
}

var _ ResponseWriter = (*Response)(nil)

// Response represents a response
type Response struct {
	http.ResponseWriter
	written        int
	status         int
	beforeFuncs    []func(ResponseWriter)
	beforeExecuted bool
}

// Write writes bytes to HTTP endpoint
func (r *Response) Write(bs []byte) (int, error) {
	if !r.beforeExecuted {
		for _, before := range r.beforeFuncs {
			before(r)
		}
		r.beforeExecuted = true
	}
	size, err := r.ResponseWriter.Write(bs)
	r.written += size
	if err != nil {
		return size, err
	}
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return size, nil
}

func (r *Response) WrittenSize() int {
	return r.written
}

// WriteHeader write status code
func (r *Response) WriteHeader(statusCode int) {
	if !r.beforeExecuted {
		for _, before := range r.beforeFuncs {
			before(r)
		}
		r.beforeExecuted = true
	}
	if r.status == 0 {
		r.status = statusCode
		r.ResponseWriter.WriteHeader(statusCode)
	}
}

// Flush flushes cached data
func (r *Response) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// WrittenStatus returned status code written
func (r *Response) WrittenStatus() int {
	return r.status
}

// Before allows for a function to be called before the ResponseWriter has been written to. This is
// useful for setting headers or any other operations that must happen before a response has been written.
func (r *Response) Before(fn func(ResponseWriter)) {
	r.beforeFuncs = append(r.beforeFuncs, fn)
}

func WrapResponseWriter(resp http.ResponseWriter) *Response {
	if v, ok := resp.(*Response); ok {
		return v
	}
	return &Response{ResponseWriter: resp}
}
