// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"
	shared_actions "gitea.dev/routers/web/shared/actions"
	"gitea.dev/services/contexttest"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/contexts"
)

type runnerMaintenanceFailure struct {
	kind     string
	armed    atomic.Bool
	observed atomic.Bool
}

func (h *runnerMaintenanceFailure) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	sql := strings.NewReplacer("`", "", "\"", "").Replace(c.SQL)
	match := strings.Contains(sql, " action_runner ")
	switch h.kind {
	case "find":
		match = match && strings.HasPrefix(sql, "SELECT")
	case "disable":
		match = match && strings.HasPrefix(sql, "UPDATE") && strings.Contains(sql, "is_disabled") && !strings.Contains(sql, "is_disabled = is_disabled")
	case "delete":
		match = match && (strings.HasPrefix(sql, "DELETE") || strings.HasPrefix(sql, "UPDATE") && strings.Contains(sql, "deleted"))
	}
	if match && h.armed.CompareAndSwap(true, false) {
		h.observed.Store(true)
		return c.Ctx, errors.New("injected runner maintenance database failure")
	}
	return c.Ctx, nil
}

func (*runnerMaintenanceFailure) AfterProcess(*contexts.ContextHook) error { return nil }

func TestRunnerBulkMaintenanceFailures(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	for _, tc := range []struct {
		name        string
		userContext bool
		status      int
	}{
		{"invalid settings context", false, http.StatusInternalServerError},
		{"non-admin settings context", true, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, response := contexttest.MockContext(t, "POST /-/admin/actions/runners/bulk")
			if tc.userContext {
				ctx.Data["PageIsUserSettings"] = true
				ctx.Doer = &user_model.User{ID: 2}
			}
			shared_actions.RunnerBulkActionPost(ctx)
			assert.Equal(t, tc.status, response.Code)
		})
	}
	for _, tc := range []struct {
		kind, action string
		status       int
	}{
		{"find", "disable", http.StatusInternalServerError},
		{"disable", "disable", http.StatusOK},
		{"delete", "delete", http.StatusOK},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			runner := &actions_model.ActionRunner{UUID: "barrier-failure-" + tc.kind, Name: "barrier-failure-" + tc.kind}
			runner.GenerateAndFillToken()
			require.NoError(t, actions_model.CreateRunner(t.Context(), runner))
			session := loginUser(t, "user1")
			hook := &runnerMaintenanceFailure{kind: tc.kind}
			hook.armed.Store(true)
			unittest.GetXORMEngine().AddHook(hook)
			req := NewRequestWithValues(t, "POST", "/-/admin/actions/runners/bulk", map[string]string{"action": tc.action, "ids": strings.Join(base.Int64sToStrings([]int64{runner.ID}), ",")})
			session.MakeRequest(t, req, tc.status)
			require.True(t, hook.observed.Load())
			actual := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: runner.ID})
			assert.False(t, actual.IsDisabled)
		})
	}
}
