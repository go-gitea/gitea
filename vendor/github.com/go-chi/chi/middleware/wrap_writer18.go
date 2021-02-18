// +build go1.8 appengine

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
		_, ps := w.(http.Pusher)
		if cn && fl && ps {
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

func (f *http2FancyWriter) Push(target string, opts *http.PushOptions) error {
	return f.basicWriter.ResponseWriter.(http.Pusher).Push(target, opts)
}

var _ http.Pusher = &http2FancyWriter{}
