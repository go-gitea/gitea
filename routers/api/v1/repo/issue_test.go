// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestLoadAssigneeUserIDs(t *testing.T) {
	unittest.PrepareTestEnv(t)

	for _, test := range []struct {
		name              string
		oneAssignee       string
		multipleAssignees []string
		wantIDs           []int64
		wantStatus        int
	}{
		{name: "empty", wantIDs: []int64{}, wantStatus: http.StatusOK},
		{name: "combined", oneAssignee: "User2", multipleAssignees: []string{"user2", "USER2", "user1", ""}, wantIDs: []int64{1, 2}, wantStatus: http.StatusOK},
		{name: "unknown user", oneAssignee: "does-not-exist", wantStatus: http.StatusUnprocessableEntity},
		{name: "no access", oneAssignee: "user4", wantStatus: http.StatusUnprocessableEntity},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, resp := contexttest.MockAPIContext(t, "user2/repo2")
			contexttest.LoadRepo(t, ctx, 2)

			ids := loadAssigneeUserIDs(ctx, test.oneAssignee, test.multipleAssignees)
			assert.Equal(t, test.wantStatus, resp.Code)
			assert.ElementsMatch(t, test.wantIDs, ids)
		})
	}
}
