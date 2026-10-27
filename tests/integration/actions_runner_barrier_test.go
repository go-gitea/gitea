// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"errors"
	"net/http"
	"slices"
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

type runnerMaintenanceFailureKey struct{}

type runnerMaintenanceFailure struct {
	kind     string
	runnerID int64
	armed    atomic.Bool
	observed atomic.Bool
}

func (h *runnerMaintenanceFailure) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	if !slices.Contains(c.Args, any(h.runnerID)) || h.kind != "delete" && c.Ctx.Value(runnerMaintenanceFailureKey{}) != h {
		return c.Ctx, nil
	}
	sql := strings.NewReplacer("`", "", "\"", "", "[", "", "]", "").Replace(c.SQL)
	match := strings.Contains(sql, "action_runner ")
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
			other := &actions_model.ActionRunner{UUID: "barrier-unrelated-" + tc.kind, Name: "barrier-unrelated-" + tc.kind}
			other.GenerateAndFillToken()
			require.NoError(t, actions_model.CreateRunner(t.Context(), other))
			session := loginUser(t, "user1")
			hook := &runnerMaintenanceFailure{kind: tc.kind, runnerID: runner.ID}
			t.Cleanup(func() { hook.armed.Store(false) })
			unittest.GetXORMEngine().AddHook(hook)
			hook.armed.Store(true)
			unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: runner.ID})
			_, err := actions_model.GetRunnerByID(context.WithValue(t.Context(), runnerMaintenanceFailureKey{}, hook), other.ID)
			require.NoError(t, err)
			require.False(t, hook.observed.Load())
			req := NewRequestWithValues(t, "POST", "/-/admin/actions/runners/bulk", map[string]string{"action": tc.action, "ids": strings.Join(base.Int64sToStrings([]int64{runner.ID}), ",")})
			req.Request = req.WithContext(context.WithValue(req.Context(), runnerMaintenanceFailureKey{}, hook))
			session.MakeRequest(t, req, tc.status)
			hook.armed.Store(false)
			require.True(t, hook.observed.Load())
			actual := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: runner.ID})
			assert.False(t, actual.IsDisabled)
		})
	}
}
