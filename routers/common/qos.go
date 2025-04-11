// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web/middleware"
	giteacontext "code.gitea.io/gitea/services/context"

	"github.com/bohde/codel"
	"github.com/go-chi/chi/v5"
)

const tplStatus503 templates.TplName = "status/503"

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
	if !setting.Service.QoS.Enabled {
		return nil
	}

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

			priority := requestPriority(ctx)

			// Check if the request can begin processing.
			err := c.Acquire(ctx, int(priority))
			if err != nil {
				log.Error("QoS error, dropping request of priority %s: %v", priority, err)
				renderServiceUnavailable(w, req)
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

// requestPriority assigns a priority value for a request based upon
// whether the user is logged in and how expensive the endpoint is
func requestPriority(ctx context.Context) Priority {
	// If the user is logged in, assign high priority.
	data := middleware.GetContextData(ctx)
	if _, ok := data[middleware.ContextDataKeySignedUser].(*user_model.User); ok {
		return HighPriority
	}

	rctx := chi.RouteContext(ctx)
	if rctx == nil {
		return DefaultPriority
	}

	// If we're operating in the context of a repo, assign low priority
	routePattern := rctx.RoutePattern()
	if strings.HasPrefix(routePattern, "/{username}/{reponame}/") {
		return LowPriority
	}

	return DefaultPriority
}

// renderServiceUnavailable will render an HTTP 503 Service
// Unavailable page, providing HTML if the client accepts it.
func renderServiceUnavailable(w http.ResponseWriter, req *http.Request) {
	acceptsHTML := false
	for _, part := range req.Header["Accept"] {
		if strings.Contains(part, "text/html") {
			acceptsHTML = true
			break
		}
	}

	// If the client doesn't accept HTML, then render a plain text response
	if !acceptsHTML {
		http.Error(w, "503 Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	tmplCtx := giteacontext.TemplateContext{}
	tmplCtx["Locale"] = middleware.Locale(w, req)
	ctxData := middleware.GetContextData(req.Context())
	err := templates.HTMLRenderer().HTML(w, http.StatusServiceUnavailable, tplStatus503, ctxData, tmplCtx)
	if err != nil {
		log.Error("Error occurs again when rendering service unavailable page: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error, please collect error logs and report to Gitea issue tracker"))
	}
}
