// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
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
	cmd := newRepoConvertToMirrorCommand()
	cmd.Writer = io.Discard
	require.ErrorContains(t, cmd.Run(ctx, []string{"repo-convert-to-mirror", "invalid"}), "{owner}/{repo}")

	owner := &user_model.User{
		Name:      "mirror-owner",
		LowerName: "mirror-owner",
	}
	require.NoError(t, db.Insert(ctx, owner))
	defer func() {
		_, err := db.GetEngine(ctx).ID(owner.ID).Delete(&user_model.User{})
		require.NoError(t, err)
	}()

	for _, testCase := range []struct {
		name           string
		existingOrigin bool
	}{
		{name: "without-origin"},
		{name: "with-origin", existingOrigin: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := t.Context()
			repo := &repo_model.Repository{
				OwnerID:          owner.ID,
				OwnerName:        owner.Name,
				Name:             testCase.name,
				LowerName:        testCase.name,
				ObjectFormatName: "sha1",
				IsArchived:       true,
			}
			require.NoError(t, db.Insert(ctx, repo))
			defer func() {
				require.NoError(t, repo_model.DeleteMirrorByRepoID(ctx, repo.ID))
				require.NoError(t, git.DeleteRepository(ctx, repo))
				_, err := db.GetEngine(ctx).ID(repo.ID).Delete(&repo_model.Repository{})
				require.NoError(t, err)
			}()
			require.NoError(t, git.InitRepository(ctx, repo, repo.ObjectFormatName, ""))
			if testCase.existingOrigin {
				require.NoError(t, git.ManagedRemoteAdd(ctx, repo, "origin", "https://example.com/original.git"))
				_, _, err := gitcmd.NewCommand("config", "--add", "remote.origin.url", "https://example.com/other.git").WithRepo(repo).RunStdString(ctx)
				require.NoError(t, err)
				_, _, err = gitcmd.NewCommand("config", "--add", "remote.origin.fetch", "+refs/pull/*:refs/pull/*").WithRepo(repo).RunStdString(ctx)
				require.NoError(t, err)
			}

			archivedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
			configBefore, _, err := gitcmd.NewCommand("config", "--local", "--list").WithRepo(repo).RunStdString(ctx)
			require.NoError(t, err)
			convert := func() error {
				cmd := newRepoConvertToMirrorCommand()
				cmd.Writer = io.Discard
				return cmd.Run(ctx, []string{"repo-convert-to-mirror", repo.FullName()})
			}
			require.ErrorAs(t, convert(), &repo_model.ErrRepoIsArchived{})
			require.Equal(t, archivedRepo, unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID}))
			unittest.AssertNotExistsBean(t, &repo_model.Mirror{RepoID: repo.ID})
			configAfter, _, err := gitcmd.NewCommand("config", "--local", "--list").WithRepo(repo).RunStdString(ctx)
			require.NoError(t, err)
			require.Equal(t, configBefore, configAfter)

			repo.IsArchived = false
			require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_archived"))
			require.NoError(t, convert())
			require.True(t, unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID}).IsMirror)
			mirror := unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: repo.ID})
			require.True(t, mirror.EnablePrune)
			require.Empty(t, mirror.RemoteAddress)
			fetchRefspec, _, err := gitcmd.NewCommand("config", "--get-all", "remote.origin.fetch").WithRepo(repo).RunStdString(ctx)
			require.NoError(t, err)
			require.Equal(t, "+refs/*:refs/*\n", fetchRefspec)
			remoteURL, _, err := gitcmd.NewCommand("config", "--get-all", "remote.origin.url").WithRepo(repo).RunStdString(ctx)
			require.NoError(t, err)
			require.Equal(t, "\n", remoteURL)

			require.ErrorContains(t, convert(), "is already a mirror")
			unittest.AssertCount(t, &repo_model.Mirror{RepoID: repo.ID}, 1)
		})
	}
}
