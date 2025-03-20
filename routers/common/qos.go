// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"net/http"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"

	"github.com/bohde/codel"
)

type Priority int

func (p Priority) String() string {
	switch p {
	case HighPriority:
		return "high"
	case DefaultPriority:
		return "default"
	case LowPriority:
		return "low"
	default:
		return fmt.Sprintf("%d", p)
	}
}

const (
	LowPriority     = Priority(-10)
	DefaultPriority = Priority(0)
	HighPriority    = Priority(10)
)

// QoS implements quality of service for requests, based upon whether
// or not the user is logged in. All traffic may get dropped, and
// anonymous users are deprioritized.
func QoS() func(next http.Handler) http.Handler {
	maxOutstanding := setting.Service.QoS.MaxInFlightRequests
	if maxOutstanding <= 0 {
		maxOutstanding = 10
	}

	c := codel.NewPriority(codel.Options{
		// The maximum number of waiting requests.
		MaxPending: setting.Service.QoS.MaxWaitingRequests,
		// The maximum number of in-flight requests.
		MaxOutstanding: maxOutstanding,
		// The target latency that a blocked request should wait
		// for. After this, it might be dropped.
		TargetLatency: setting.Service.QoS.TargetWaitTime,
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			priority := DefaultPriority

			// If the user is logged in, assign high priority.
			data := middleware.GetContextData(req.Context())
			if _, ok := data[middleware.ContextDataKeySignedUser].(*user_model.User); ok {
				priority = HighPriority
			} else if IsGitContents(req.URL.Path) {
				// Otherwise, if the path would is accessing git contents directly, mark as low priority
				priority = LowPriority
			}

			// Check if the request can begin processing.
			err := c.Acquire(ctx, int(priority))
			if err != nil {
				// If it failed, the service is over capacity and should error
				http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
				return
			}

			// Release long-polling immediately, so they don't always
			// take up an in-flight request
			if strings.Contains(req.URL.Path, "/user/events") {
				c.Release()
			} else {
				defer c.Release()
			}

			next.ServeHTTP(w, req)
		})
	}
}

func IsGitContents(path string) bool {
	parts := []string{
		"refs",
		"archive",
		"commit",
		"graph",
		"blame",
		"branches",
		"tags",
		"labels",
		"stars",
		"search",
		"activity",
		"wiki",
		"watchers",
		"compare",
		"raw",
		"src",
		"commits",
	}

	for _, p := range parts {
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}
