// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"net/http"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

// TestFindColumn covers the scoping rule every board handler depends on: a request only
// resolves projects owned by the scope its route assigned, so an ID belonging to anyone
// else reads as not found rather than leaking across owners.
func TestFindColumn(t *testing.T) {
	for _, tc := range []struct {
		name       string
		projectID  string
		columnID   string
		doerID     int64
		repoScoped bool
		resolves   bool
	}{
		{"repository project", "1", "2", 2, true, true},
		{"owner project", "4", "4", 2, false, true},
		{"repository board cannot reach an owner project", "4", "1", 2, true, false},
		{"owner board cannot reach another owner's project", "4", "4", 1, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unittest.PrepareTestEnv(t)
			ctx, _ := contexttest.MockContext(t, "user2/-/projects")
			contexttest.LoadUser(t, ctx, tc.doerID)
			if tc.repoScoped {
				contexttest.LoadRepo(t, ctx, 1)
			} else {
				ctx.ContextUser = ctx.Doer
			}
			ctx.SetPathParam("id", tc.projectID)
			ctx.SetPathParam("columnID", tc.columnID)

			project, column := findColumn(ctx)
			assert.Equal(t, tc.resolves, project != nil)
			assert.Equal(t, tc.resolves, column != nil)
			if tc.resolves {
				assert.False(t, ctx.Written())
			} else {
				// a foreign ID must read as not found, never as a 500
				assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
			}
		})
	}
}
