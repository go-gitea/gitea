package certmagic

import (
	"context"
	"errors"
	"log"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"
)

var jm = &jobManager{maxConcurrentJobs: 1000}

type jobManager struct {
	mu                sync.Mutex
	maxConcurrentJobs int
	activeWorkers     int
	queue             []namedJob
	names             map[string]struct{}
}

type namedJob struct {
	name   string
	job    func() error
	logger *zap.Logger
}

// Submit enqueues the given job with the given name. If name is non-empty
// and a job with the same name is already enqueued or running, this is a
// no-op. If name is empty, no duplicate prevention will occur. The job
// manager will then run this job as soon as it is able.
func (jm *jobManager) Submit(logger *zap.Logger, name string, job func() error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if jm.names == nil {
		jm.names = make(map[string]struct{})
	}
	if name != "" {
		// prevent duplicate jobs
		if _, ok := jm.names[name]; ok {
			return
		}
		jm.names[name] = struct{}{}
	}
	jm.queue = append(jm.queue, namedJob{name, job, logger})
	if jm.activeWorkers < jm.maxConcurrentJobs {
		jm.activeWorkers++
		go jm.worker()
	}
}

func (jm *jobManager) worker() {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, stackTraceBufferSize)
			buf = buf[:runtime.Stack(buf, false)]
			log.Printf("panic: certificate worker: %v\n%s", err, buf)
		}
	}()

	for {
		jm.mu.Lock()
		if len(jm.queue) == 0 {
			jm.activeWorkers--
			jm.mu.Unlock()
			return
		}
		next := jm.queue[0]
		jm.queue = jm.queue[1:]
		jm.mu.Unlock()
		if err := next.job(); err != nil {
			if next.logger != nil {
				next.logger.Error("job failed", zap.Error(err))
			}
		}
		if next.name != "" {
			jm.mu.Lock()
			delete(jm.names, next.name)
			jm.mu.Unlock()
		}
	}
}

func doWithRetry(ctx context.Context, log *zap.Logger, f func(context.Context) error) error {
	var attempts int
	ctx = context.WithValue(ctx, AttemptsCtxKey, &attempts)

	// the initial intervalIndex is -1, signaling
	// that we should not wait for the first attempt
	start, intervalIndex := time.Now(), -1
	var err error

	for time.Since(start) < maxRetryDuration {
		var wait time.Duration
		if intervalIndex >= 0 {
			wait = retryIntervals[intervalIndex]
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return context.Canceled
		case <-timer.C:
			err = f(ctx)
			attempts++
			if err == nil || errors.Is(err, context.Canceled) {
				return err
			}
			var errNoRetry ErrNoRetry
			if errors.As(err, &errNoRetry) {
				return err
			}
			if intervalIndex < len(retryIntervals)-1 {
				intervalIndex++
			}
			if time.Since(start) < maxRetryDuration {
				if log != nil {
					log.Error("will retry",
						zap.Error(err),
						zap.Int("attempt", attempts),
						zap.Duration("retrying_in", retryIntervals[intervalIndex]),
						zap.Duration("elapsed", time.Since(start)),
						zap.Duration("max_duration", maxRetryDuration))
				}
			} else {
				if log != nil {
					log.Error("final attempt; giving up",
						zap.Error(err),
						zap.Int("attempt", attempts),
						zap.Duration("elapsed", time.Since(start)),
						zap.Duration("max_duration", maxRetryDuration))
				}
				return nil
			}
		}
	}
	return err
}

// ErrNoRetry is an error type which signals
// to stop retries early.
type ErrNoRetry struct{ Err error }

// Unwrap makes it so that e wraps e.Err.
func (e ErrNoRetry) Unwrap() error { return e.Err }
func (e ErrNoRetry) Error() string { return e.Err.Error() }

type retryStateCtxKey struct{}

// AttemptsCtxKey is the context key for the value
// that holds the attempt counter. The value counts
// how many times the operation has been attempted.
// A value of 0 means first attempt.
var AttemptsCtxKey retryStateCtxKey

// retryIntervals are based on the idea of exponential
// backoff, but weighed a little more heavily to the
// front. We figure that intermittent errors would be
// resolved after the first retry, but any errors after
// that would probably require at least a few minutes
// to clear up: either for DNS to propagate, for the
// administrator to fix their DNS or network properties,
// or some other external factor needs to change. We
// chose intervals that we think will be most useful
// without introducing unnecessary delay. The last
// interval in this list will be used until the time
// of maxRetryDuration has elapsed.
var retryIntervals = []time.Duration{
	1 * time.Minute,
	2 * time.Minute,
	2 * time.Minute,
	5 * time.Minute, // elapsed: 10 min
	10 * time.Minute,
	20 * time.Minute,
	20 * time.Minute, // elapsed: 1 hr
	30 * time.Minute,
	30 * time.Minute, // elapsed: 2 hr
	1 * time.Hour,
	3 * time.Hour, // elapsed: 6 hr
	6 * time.Hour, // for up to maxRetryDuration
}

// maxRetryDuration is the maximum duration to try
// doing retries using the above intervals.
const maxRetryDuration = 24 * time.Hour * 30
