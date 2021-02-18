// +build go1.8 appengine

package middleware

import (
	"errors"
	"net/http"
)

func (w *maybeCompressResponseWriter) Push(target string, opts *http.PushOptions) error {
	if ps, ok := w.w.(http.Pusher); ok {
		return ps.Push(target, opts)
	}
	return errors.New("chi/middleware: http.Pusher is unavailable on the writer")
}
