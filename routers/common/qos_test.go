// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"testing"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestRequestPriority(t *testing.T) {
	type test struct {
		Name         string
		User         *user_model.User
		RoutePattern string
		Expected     Priority
	}

	cases := []test{
		{
			Name:     "Logged In",
			User:     &user_model.User{},
			Expected: HighPriority,
		},
		{
			Name:         "Sign In",
			RoutePattern: "/user/login",
			Expected:     DefaultPriority,
		},
		{
			Name:         "Repo Home",
			RoutePattern: "/{username}/{reponame}",
			Expected:     DefaultPriority,
		},
		{
			Name:         "User Repo",
			RoutePattern: "/{username}/{reponame}/src/branch/main",
			Expected:     LowPriority,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, _ := contexttest.MockContext(t, "")

			if tc.User != nil {
				data := middleware.GetContextData(ctx)
				data[middleware.ContextDataKeySignedUser] = tc.User
			}

			rctx := chi.RouteContext(ctx)
			rctx.RoutePatterns = []string{tc.RoutePattern}

			assert.Exactly(t, tc.Expected, requestPriority(ctx))
		})
	}
}

func TestRenderServiceUnavailable(t *testing.T) {
	t.Run("HTML", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "")
		ctx.Req.Header.Set("Accept", "text/html")

		renderServiceUnavailable(resp, ctx.Req)
		assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
		assert.Contains(t, resp.Header().Get("Content-Type"), "text/html")

		body := resp.Body.String()
		assert.Contains(t, body, `lang="en-US"`)
		assert.Contains(t, body, "503 Service Unavailable")
	})

	t.Run("plain", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "")
		ctx.Req.Header.Set("Accept", "text/plain")

		renderServiceUnavailable(resp, ctx.Req)
		assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
		assert.Contains(t, resp.Header().Get("Content-Type"), "text/plain")

		body := resp.Body.String()
		assert.Contains(t, body, "503 Service Unavailable")
	})
}
