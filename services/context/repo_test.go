// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"testing"

	repo_model "gitea.dev/models/repo"

	"github.com/stretchr/testify/assert"
)

func TestCompareHeadRef(t *testing.T) {
	baseRepo := &repo_model.Repository{ID: 1, OwnerID: 100, OwnerName: "base-owner", Name: "base-repo"}
	assert.Equal(t, "my-branch", CompareHeadRef(baseRepo, baseRepo, "my-branch"))
	assert.Equal(t, "head-owner/head-repo:my-branch", CompareHeadRef(baseRepo, &repo_model.Repository{ID: 2, OwnerID: 100, OwnerName: "head-owner", Name: "head-repo"}, "my-branch"))
	assert.Equal(t, "head-owner:my-branch", CompareHeadRef(baseRepo, &repo_model.Repository{ID: 2, OwnerID: 101, OwnerName: "head-owner", Name: "head-repo"}, "my-branch"))
}
