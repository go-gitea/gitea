// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestCompareHeadRef(t *testing.T) {
	defer test.MockVariableValue(&setting.Repository.AllowForkIntoSameOwner)()
	headRepo := &repo_model.Repository{OwnerName: "user", Name: "fork"}

	setting.Repository.AllowForkIntoSameOwner = false
	assert.Equal(t, "user:my-branch", CompareHeadRef(headRepo, "my-branch"))

	setting.Repository.AllowForkIntoSameOwner = true
	assert.Equal(t, "user/fork:my-branch", CompareHeadRef(headRepo, "my-branch"))
}
