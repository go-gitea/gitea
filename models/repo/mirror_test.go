// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPersonalIssueState(t *testing.T) {
	for path, want := range map[string]bool{
		"/owner/repo/issues/12/watch":                              true,
		"/owner/repo/pulls/12/viewed-files":                        true,
		"/api/v1/repos/owner/repo/issues/12/subscriptions/someone": true,
		"/api/v1/repos/owner/repo/issues/12/subscriptions":         true,
		"/owner/subscriptions-mirror/issues/12/comments":           false,
		"/api/v1/repos/owner/subscriptions-mirror/issues/12":       false,
		"/api/v1/repos/owner/repo/topics/watch":                    false,
		"/owner/repo/issues/12/title":                              false,
		"/owner/repo/watch":                                        false,
	} {
		assert.Equal(t, want, IsPersonalIssueState(path), path)
	}
}
