// +build go1.7,!go1.8

package middleware

import (
	"io"
	"net/http"
)

// NewWrapResponseWriter wraps an http.ResponseWriter, returning a proxy that allows you to
// hook into various parts of the response process.
func NewWrapResponseWriter(w http.ResponseWriter, protoMajor int) WrapResponseWriter {
	_, cn := w.(http.CloseNotifier)
	_, fl := w.(http.Flusher)

	bw := basicWriter{ResponseWriter: w}

	if protoMajor == 2 {
		if cn && fl {
			return &http2FancyWriter{bw}
		}
	} else {
		_, hj := w.(http.Hijacker)
		_, rf := w.(io.ReaderFrom)
		if cn && fl && hj && rf {
			return &httpFancyWriter{bw}
		}
	}
	if fl {
		return &flushWriter{bw}
	}

	return &bw
}
