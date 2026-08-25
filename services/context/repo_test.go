// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"github.com/stretchr/testify/assert"
)

func TestCompareHeadRef(t *testing.T) {
	headRepo := &repo_model.Repository{OwnerName: "user", Name: "fork"}
	assert.Equal(t, "user/fork:my-branch", CompareHeadRef(headRepo, "my-branch"))
}
