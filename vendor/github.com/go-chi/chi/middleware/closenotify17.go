// +build go1.7,!go1.8

package middleware

import (
	"context"
	"net/http"
)

// CloseNotify is a middleware that cancels ctx when the underlying
// connection has gone away. It can be used to cancel long operations
// on the server when the client disconnects before the response is ready.
//
// Note: this behaviour is standard in Go 1.8+, so the middleware does nothing
// on 1.8+ and exists just for backwards compatibility.
func CloseNotify(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		cn, ok := w.(http.CloseNotifier)
		if !ok {
			panic("chi/middleware: CloseNotify expects http.ResponseWriter to implement http.CloseNotifier interface")
		}
		closeNotifyCh := cn.CloseNotify()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		go func() {
			select {
			case <-ctx.Done():
				return
			case <-closeNotifyCh:
				cancel()
				return
			}
		}()

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
