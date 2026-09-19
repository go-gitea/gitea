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
	defer test.MockVariableValue(&setting.Repository.AllowForkIntoSameOwner, false)()
	baseRepo := &repo_model.Repository{ID: 1, OwnerID: 100, OwnerName: "base-owner", Name: "base-repo"}
	sameRepo := baseRepo
	sameOwner := &repo_model.Repository{ID: 2, OwnerID: 100, OwnerName: "head-owner", Name: "head-repo"}
	diffOwner := &repo_model.Repository{ID: 2, OwnerID: 101, OwnerName: "head-owner", Name: "head-repo"}

	assert.Equal(t, "my-branch", CompareHeadRef(baseRepo, sameRepo, "my-branch"))
	assert.Equal(t, "head-owner/head-repo:my-branch", CompareHeadRef(baseRepo, sameOwner, "my-branch"))
	assert.Equal(t, "head-owner:my-branch", CompareHeadRef(baseRepo, diffOwner, "my-branch"))
	setting.Repository.AllowForkIntoSameOwner = true
	assert.Equal(t, "head-owner/head-repo:my-branch", CompareHeadRef(baseRepo, diffOwner, "my-branch"))
}
