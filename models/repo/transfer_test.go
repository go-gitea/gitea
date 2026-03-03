// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"

	"github.com/stretchr/testify/assert"
)

func TestRepoTransfer_IsReparent(t *testing.T) {
	// A normal transfer (recipient != repo.OwnerID)
	transfer := &repo_model.RepoTransfer{
		RecipientID: 2,
		Repo: &repo_model.Repository{
			OwnerID: 1,
		},
	}
	assert.False(t, transfer.IsReparent(t.Context()))

	// A reparent request (recipient == repo.OwnerID)
	reparent := &repo_model.RepoTransfer{
		RecipientID: 1,
		Repo: &repo_model.Repository{
			OwnerID: 1,
		},
	}
	assert.True(t, reparent.IsReparent(t.Context()))
}

func TestRepoTransfer_GetTargetOwnerID(t *testing.T) {
	transfer := &repo_model.RepoTransfer{
		TeamIDs: []int64{42},
	}
	assert.Equal(t, int64(42), transfer.GetTargetOwnerID())

	transferEmpty := &repo_model.RepoTransfer{
		TeamIDs: []int64{},
	}
	assert.Equal(t, int64(0), transferEmpty.GetTargetOwnerID())
}
