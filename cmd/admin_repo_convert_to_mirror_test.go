// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
	"strings"
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/require"
)

func TestAdminRepoConvertToMirror(t *testing.T) {
	ctx := t.Context()
	owner := &user_model.User{
		Name:        "mirror-owner",
		LowerName:   "mirror-owner",
		Email:       "mirror-owner@gitea.local",
		Passwd:      "unused",
		Avatar:      "",
		AvatarEmail: "mirror-owner@gitea.local",
	}
	require.NoError(t, db.Insert(ctx, owner))
	repo := &repo_model.Repository{
		OwnerID:          owner.ID,
		OwnerName:        owner.Name,
		Name:             "test-mirror",
		LowerName:        "test-mirror",
		ObjectFormatName: "sha1",
	}
	require.NoError(t, db.Insert(ctx, repo))
	defer func() {
		require.NoError(t, repo_model.DeleteMirrorByRepoID(ctx, repo.ID))
		require.NoError(t, git.DeleteRepository(ctx, repo))
		_, err := db.GetEngine(ctx).ID(repo.ID).Delete(&repo_model.Repository{})
		require.NoError(t, err)
		_, err = db.GetEngine(ctx).ID(owner.ID).Delete(&user_model.User{})
		require.NoError(t, err)
	}()
	require.NoError(t, git.InitRepository(ctx, repo, repo.ObjectFormatName, ""))

	cmd := newRepoConvertToMirrorCommand()
	cmd.Writer = io.Discard
	err := cmd.Run(ctx, []string{"repo-convert-to-mirror", "invalid"})
	require.ErrorContains(t, err, "{owner}/{repo}")
	require.NoError(t, cmd.Run(ctx, []string{"repo-convert-to-mirror", "mirror-owner/test-mirror"}))

	convertedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID, IsMirror: true})
	mirror, err := repo_model.GetMirrorByRepoID(ctx, repo.ID)
	require.NoError(t, err)
	require.Equal(t, repo.ID, mirror.RepoID)
	require.True(t, mirror.EnablePrune)
	require.Empty(t, mirror.RemoteAddress)
	fetchRefspec, _, err := gitcmd.NewCommand("config", "--get", "remote.origin.fetch").WithRepo(repo).RunStdString(ctx)
	require.NoError(t, err)
	require.Equal(t, "+refs/*:refs/*", strings.TrimSpace(fetchRefspec))

	cmd = newRepoConvertToMirrorCommand()
	cmd.Writer = io.Discard
	err = cmd.Run(ctx, []string{"repo-convert-to-mirror", "mirror-owner/test-mirror"})
	require.ErrorContains(t, err, "is already a mirror")

	count, err := db.GetEngine(ctx).Count(&repo_model.Mirror{RepoID: convertedRepo.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
}
