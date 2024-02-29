// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"net/url"
	"testing"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/private"

	"github.com/stretchr/testify/assert"
)

func TestAPIPrivateNoServ(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		key, user, err := private.ServNoCommand(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), user.ID)
		assert.Equal(t, "user2", user.Name)
		assert.Equal(t, int64(1), key.ID)
		assert.Equal(t, "user2@localhost", key.Name)

		deployKey, err := asymkey_model.AddDeployKey(ctx, 1, "test-deploy", "sk-ecdsa-sha2-nistp256@openssh.com AAAAInNrLWVjZHNhLXNoYTItbmlzdHAyNTZAb3BlbnNzaC5jb20AAAAIbmlzdHAyNTYAAABBBGXEEzWmm1dxb+57RoK5KVCL0w2eNv9cqJX2AGGVlkFsVDhOXHzsadS3LTK4VlEbbrDMJdoti9yM8vclA8IeRacAAAAEc3NoOg== nocomment", false)
		assert.NoError(t, err)

		key, user, err = private.ServNoCommand(ctx, deployKey.KeyID)
		assert.NoError(t, err)
		assert.Empty(t, user)
		assert.Equal(t, deployKey.KeyID, key.ID)
		assert.Equal(t, "test-deploy", key.Name)
	})
}

func TestAPIPrivateServ(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Can push to a repo we own
		results, extra := private.ServCommand(ctx, 1, "user2", "repo1", perm.AccessModeWrite, "git-upload-pack", "")
		assert.NoError(t, extra.Error)
		assert.False(t, results.IsWiki)
		assert.Zero(t, results.DeployKeyID)
		assert.Equal(t, int64(1), results.KeyID)
		assert.Equal(t, "user2@localhost", results.KeyName)
		assert.Equal(t, "user2", results.UserName)
		assert.Equal(t, int64(2), results.UserID)
		assert.Equal(t, "user2", results.OwnerName)
		assert.Equal(t, "repo1", results.RepoName)
		assert.Equal(t, int64(1), results.RepoID)

		// Cannot push to a private repo we're not associated with
		results, extra = private.ServCommand(ctx, 1, "user15", "big_test_private_1", perm.AccessModeWrite, "git-upload-pack", "")
		assert.Error(t, extra.Error)
		assert.Empty(t, results)

		// Cannot pull from a private repo we're not associated with
		results, extra = private.ServCommand(ctx, 1, "user15", "big_test_private_1", perm.AccessModeRead, "git-upload-pack", "")
		assert.Error(t, extra.Error)
		assert.Empty(t, results)

		// Can pull from a public repo we're not associated with
		results, extra = private.ServCommand(ctx, 1, "user15", "big_test_public_1", perm.AccessModeRead, "git-upload-pack", "")
		assert.NoError(t, extra.Error)
		assert.False(t, results.IsWiki)
		assert.Zero(t, results.DeployKeyID)
		assert.Equal(t, int64(1), results.KeyID)
		assert.Equal(t, "user2@localhost", results.KeyName)
		assert.Equal(t, "user2", results.UserName)
		assert.Equal(t, int64(2), results.UserID)
		assert.Equal(t, "user15", results.OwnerName)
		assert.Equal(t, "big_test_public_1", results.RepoName)
		assert.Equal(t, int64(17), results.RepoID)

		// Cannot push to a public repo we're not associated with
		results, extra = private.ServCommand(ctx, 1, "user15", "big_test_public_1", perm.AccessModeWrite, "git-upload-pack", "")
		assert.Error(t, extra.Error)
		assert.Empty(t, results)

		// Add reading deploy key
		deployKey, err := asymkey_model.AddDeployKey(ctx, 19, "test-deploy", "sk-ecdsa-sha2-nistp256@openssh.com AAAAInNrLWVjZHNhLXNoYTItbmlzdHAyNTZAb3BlbnNzaC5jb20AAAAIbmlzdHAyNTYAAABBBGXEEzWmm1dxb+57RoK5KVCL0w2eNv9cqJX2AGGVlkFsVDhOXHzsadS3LTK4VlEbbrDMJdoti9yM8vclA8IeRacAAAAEc3NoOg== nocomment", true)
		assert.NoError(t, err)

		// Can pull from repo we're a deploy key for
		results, extra = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_1", perm.AccessModeRead, "git-upload-pack", "")
		assert.NoError(t, extra.Error)
		assert.False(t, results.IsWiki)
		assert.NotZero(t, results.DeployKeyID)
		assert.Equal(t, deployKey.KeyID, results.KeyID)
		assert.Equal(t, "test-deploy", results.KeyName)
		assert.Equal(t, "user15", results.UserName)
		assert.Equal(t, int64(15), results.UserID)
		assert.Equal(t, "user15", results.OwnerName)
		assert.Equal(t, "big_test_private_1", results.RepoName)
		assert.Equal(t, int64(19), results.RepoID)

		// Cannot push to a private repo with reading key
		results, extra = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_1", perm.AccessModeWrite, "git-upload-pack", "")
		assert.Error(t, extra.Error)
		assert.Empty(t, results)

		// Cannot pull from a private repo we're not associated with
		results, extra = private.ServCommand(ctx, deployKey.ID, "user15", "big_test_private_2", perm.AccessModeRead, "git-upload-pack", "")
		assert.Error(t, extra.Error)
		assert.Empty(t, results)

		// Cannot pull from a public repo we're not associated with
		results, extra = private.ServCommand(ctx, deployKey.ID, "user15", "big_test_public_1", perm.AccessModeRead, "git-upload-pack", "")
		assert.Error(t, extra.Error)
		assert.Empty(t, results)

		// Add writing deploy key
		deployKey, err = asymkey_model.AddDeployKey(ctx, 20, "test-deploy", "sk-ecdsa-sha2-nistp256@openssh.com AAAAInNrLWVjZHNhLXNoYTItbmlzdHAyNTZAb3BlbnNzaC5jb20AAAAIbmlzdHAyNTYAAABBBGXEEzWmm1dxb+57RoK5KVCL0w2eNv9cqJX2AGGVlkFsVDhOXHzsadS3LTK4VlEbbrDMJdoti9yM8vclA8IeRacAAAAEc3NoOg== nocomment", false)
		assert.NoError(t, err)

		// Cannot push to a private repo with reading key
		results, extra = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_1", perm.AccessModeWrite, "git-upload-pack", "")
		assert.Error(t, extra.Error)
		assert.Empty(t, results)

		// Can pull from repo we're a writing deploy key for
		results, extra = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_2", perm.AccessModeRead, "git-upload-pack", "")
		assert.NoError(t, extra.Error)
		assert.False(t, results.IsWiki)
		assert.NotZero(t, results.DeployKeyID)
		assert.Equal(t, deployKey.KeyID, results.KeyID)
		assert.Equal(t, "test-deploy", results.KeyName)
		assert.Equal(t, "user15", results.UserName)
		assert.Equal(t, int64(15), results.UserID)
		assert.Equal(t, "user15", results.OwnerName)
		assert.Equal(t, "big_test_private_2", results.RepoName)
		assert.Equal(t, int64(20), results.RepoID)

		// Can push to repo we're a writing deploy key for
		results, extra = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_2", perm.AccessModeWrite, "git-upload-pack", "")
		assert.NoError(t, extra.Error)
		assert.False(t, results.IsWiki)
		assert.NotZero(t, results.DeployKeyID)
		assert.Equal(t, deployKey.KeyID, results.KeyID)
		assert.Equal(t, "test-deploy", results.KeyName)
		assert.Equal(t, "user15", results.UserName)
		assert.Equal(t, int64(15), results.UserID)
		assert.Equal(t, "user15", results.OwnerName)
		assert.Equal(t, "big_test_private_2", results.RepoName)
		assert.Equal(t, int64(20), results.RepoID)
	})
}
