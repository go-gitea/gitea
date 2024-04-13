// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http"

	web_types "code.gitea.io/gitea/modules/web/types"
)

// ResponseWriter represents a response writer for HTTP
type ResponseWriter interface {
	http.ResponseWriter
	http.Flusher
	web_types.ResponseStatusProvider

	Before(func(ResponseWriter))

	Status() int // used by access logger template
	Size() int   // used by access logger template
}

var _ ResponseWriter = &Response{}

// Response represents a response
type Response struct {
	http.ResponseWriter
	written        int
	status         int
	befores        []func(ResponseWriter)
	beforeExecuted bool
}

// Write writes bytes to HTTP endpoint
func (r *Response) Write(bs []byte) (int, error) {
	if !r.beforeExecuted {
		for _, before := range r.befores {
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

func (r *Response) Status() int {
	return r.status
}

func (r *Response) Size() int {
	return r.written
}

// WriteHeader write status code
func (r *Response) WriteHeader(statusCode int) {
	if !r.beforeExecuted {
		for _, before := range r.befores {
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
func (r *Response) Before(f func(ResponseWriter)) {
	r.befores = append(r.befores, f)
}

func WrapResponseWriter(resp http.ResponseWriter) *Response {
	if v, ok := resp.(*Response); ok {
		return v
	}
	return &Response{
		ResponseWriter: resp,
		status:         0,
		befores:        make([]func(ResponseWriter), 0),
	}
}
