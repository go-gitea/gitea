// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEditorconfig(t *testing.T) {
	defer test.MockVariableValue(&setting.Git.HomePath, t.TempDir())()
	require.NoError(t, git.InitSimple())
	repoPath := t.TempDir()
	_, err := git.ForceFastImportWithInit(t.Context(), repoPath, []git.FastImportCommit{
		{Ref: "refs/heads/main", Files: []git.FastImportFile{{Path: ".editorconfig", Content: "root = true\n[*]\ntab_width = 4\n"}}},
		{Ref: "refs/heads/feature", Files: []git.FastImportFile{{Path: ".editorconfig", Content: "root = true\n[*]\ntab_width = 2\n"}}},
	})
	require.NoError(t, err)
	gitRepo, err := git.OpenRepositoryLocal(t.Context(), repoPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, gitRepo.Close()) })
	mainCommit, err := gitRepo.GetBranchCommit(t.Context(), "main")
	require.NoError(t, err)
	featureCommit, err := gitRepo.GetBranchCommit(t.Context(), "feature")
	require.NoError(t, err)

	for _, tt := range []struct {
		name     string
		commit   *git.Commit
		explicit []*git.Commit
		width    int
	}{
		{name: "default branch", width: 4},
		{name: "current commit", commit: featureCommit, width: 2},
		{name: "explicit commit", commit: featureCommit, explicit: []*git.Commit{mainCommit}, width: 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &Repository{GitRepo: gitRepo, Repository: &repo_model.Repository{DefaultBranch: "main"}, Commit: tt.commit}
			cfg, warning, err := repo.GetEditorconfig(t.Context(), tt.explicit...)
			require.NoError(t, err)
			require.NoError(t, warning)
			require.NotNil(t, cfg)
			def, err := cfg.GetDefinitionForFilename("test.txt")
			require.NoError(t, err)
			assert.Equal(t, tt.width, def.TabWidth)
		})
	}
}

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
