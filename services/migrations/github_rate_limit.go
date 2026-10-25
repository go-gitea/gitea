// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"gitea.dev/modules/log"

	"github.com/google/go-github/v92/github"
)

const githubRateLimitAttempts = 5

type githubRateLimitTransport struct {
	base http.RoundTripper
}

func (t *githubRateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.base
	if transport == nil {
		transport = http.DefaultTransport
	}
	for attempt := 0; ; attempt++ {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		resp, err := transport.RoundTrip(req.Clone(req.Context()))
		if err != nil || req.Method != http.MethodGet || (resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests) {
			return resp, err
		}
		body := resp.Body
		rateErr := github.CheckResponse(resp)
		_ = body.Close() // CheckResponse replaces the body with a readable copy.
		delay, limited := githubRateLimitDelay(rateErr, attempt, time.Now())
		if !limited || attempt+1 >= githubRateLimitAttempts {
			return resp, nil
		}
		log.Info("migration-rate-retry path=%s status=%d attempt=%d delay=%s remaining=%q reset=%q requestID=%q", req.URL.Path, resp.StatusCode, attempt+1, delay, resp.Header.Get("X-RateLimit-Remaining"), resp.Header.Get("X-RateLimit-Reset"), resp.Header.Get("X-GitHub-Request-Id"))
		_ = resp.Body.Close()
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
}

func githubRateLimitDelay(err error, attempt int, now time.Time) (time.Duration, bool) {
	delay := time.Second * time.Duration(1+attempt*attempt)
	var response *http.Response
	if primary, ok := errors.AsType[*github.RateLimitError](err); ok {
		delay = max(delay, primary.Rate.Reset.Sub(now)+time.Second)
		response = primary.Response
	} else if secondary, ok := errors.AsType[*github.AbuseRateLimitError](err); ok {
		delay = max(delay, time.Minute*time.Duration(attempt+1))
		if secondary.RetryAfter != nil {
			delay = max(time.Second, *secondary.RetryAfter)
		}
		response = secondary.Response
	} else {
		return 0, false
	}
	if response != nil {
		value := response.Header.Get("Retry-After")
		if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
			delay = max(delay, time.Duration(seconds)*time.Second)
		} else if until, err := http.ParseTime(value); err == nil {
			delay = max(delay, until.Sub(now))
		}
	}
	return delay, true
}
