package middleware

import (
	"net/http"
	"time"
)

const (
	errCapacityExceeded = "Server capacity exceeded."
	errTimedOut         = "Timed out while waiting for a pending request to complete."
	errContextCanceled  = "Context was canceled."
)

var (
	defaultBacklogTimeout = time.Second * 60
)

// Throttle is a middleware that limits number of currently processed requests
// at a time.
func Throttle(limit int) func(http.Handler) http.Handler {
	return ThrottleBacklog(limit, 0, defaultBacklogTimeout)
}

// ThrottleBacklog is a middleware that limits number of currently processed
// requests at a time and provides a backlog for holding a finite number of
// pending requests.
func ThrottleBacklog(limit int, backlogLimit int, backlogTimeout time.Duration) func(http.Handler) http.Handler {
	if limit < 1 {
		panic("chi/middleware: Throttle expects limit > 0")
	}

	if backlogLimit < 0 {
		panic("chi/middleware: Throttle expects backlogLimit to be positive")
	}

	t := throttler{
		tokens:         make(chan token, limit),
		backlogTokens:  make(chan token, limit+backlogLimit),
		backlogTimeout: backlogTimeout,
	}

	// Filling tokens.
	for i := 0; i < limit+backlogLimit; i++ {
		if i < limit {
			t.tokens <- token{}
		}
		t.backlogTokens <- token{}
	}

	fn := func(h http.Handler) http.Handler {
		t.h = h
		return &t
	}

	return fn
}

// token represents a request that is being processed.
type token struct{}

// throttler limits number of currently processed requests at a time.
type throttler struct {
	h              http.Handler
	tokens         chan token
	backlogTokens  chan token
	backlogTimeout time.Duration
}

// ServeHTTP is the primary throttler request handler
func (t *throttler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	select {
	case <-ctx.Done():
		http.Error(w, errContextCanceled, http.StatusServiceUnavailable)
		return
	case btok := <-t.backlogTokens:
		timer := time.NewTimer(t.backlogTimeout)

		defer func() {
			t.backlogTokens <- btok
		}()

		select {
		case <-timer.C:
			http.Error(w, errTimedOut, http.StatusServiceUnavailable)
			return
		case <-ctx.Done():
			http.Error(w, errContextCanceled, http.StatusServiceUnavailable)
			return
		case tok := <-t.tokens:
			defer func() {
				t.tokens <- tok
			}()
			t.h.ServeHTTP(w, r)
		}
		return
	default:
		http.Error(w, errCapacityExceeded, http.StatusServiceUnavailable)
		return
	}
}
