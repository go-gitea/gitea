// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescribeMirrorSync(t *testing.T) {
	repo := &repo_model.Repository{OwnerName: "owner", Name: "repo"}

	assert.Equal(t, "pull mirror repository owner/repo", describeMirrorSync(PullMirrorType, repo))
	assert.Equal(t, "push mirror repository owner/repo", describeMirrorSync(PushMirrorType, repo))
	assert.Equal(t, "mirror repository owner/repo", describeMirrorSync(SyncType(100), repo))
	assert.Equal(t, "pull mirror repository unknown repository", describeMirrorSync(PullMirrorType, nil))
}

func TestQueueMirrorSyncCancelledIncludesRepository(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := queueMirrorSync(ctx, &repo_model.Repository{OwnerName: "owner", Name: "repo"}, PullMirrorType, 1)
	require.Error(t, err)
	assert.True(t, db.IsErrCancelled(err))
	assert.Equal(t, "Cancelled: before queueing pull mirror repository owner/repo", err.Error())
}
