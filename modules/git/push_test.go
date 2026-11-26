// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushToExternalWithoutTracking(t *testing.T) {
	sourceCommitID, err := GetBranchCommitID(t.Context(), mockRepository("repo1_bare"), "master")
	require.NoError(t, err)

	for _, testCase := range []struct {
		name             string
		destinationCount int
		pushURLs         bool
		rewrite          bool
	}{
		{name: "url", destinationCount: 1},
		{name: "rewritten urls", destinationCount: 2, rewrite: true},
		{name: "rewritten pushurls", destinationCount: 2, pushURLs: true, rewrite: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			sourcePath := t.TempDir()
			require.NoError(t, CloneRepoToLocal(t.Context(), mockRepository("repo1_bare"), sourcePath, CloneRepoOptions{Bare: true}))
			sourceRepo := gitrepo.RepositoryManaged("push-test", sourcePath)
			remoteURLKey := "remote.mirror.name.url"
			if testCase.pushURLs {
				require.NoError(t, ManagedConfigAdd(t.Context(), sourceRepo, remoteURLKey, filepath.Join(t.TempDir(), "unused.git")))
				remoteURLKey = "remote.mirror.name.pushurl"
			}
			var destinationPaths []string
			for range testCase.destinationCount {
				destinationRoot := t.TempDir()
				destinationPath := filepath.Join(destinationRoot, "prefix", "repo.git")
				require.NoError(t, InitRepositoryLocal(t.Context(), destinationPath, true, Sha1ObjectFormat.Name(), "master"))
				remoteURL := destinationPath
				if testCase.rewrite {
					remoteURL = "file://" + destinationRoot + "/repo.git"
					require.NoError(t, ManagedConfigAdd(t.Context(), sourceRepo, "url.file://"+destinationRoot+"/prefix/.insteadOf", "file://"+destinationRoot+"/"))
				}
				require.NoError(t, ManagedConfigAdd(t.Context(), sourceRepo, remoteURLKey, remoteURL))
				destinationPaths = append(destinationPaths, destinationPath)
			}
			require.NoError(t, PushToExternalWithoutTracking(t.Context(), sourceRepo, []string{"+refs/heads/*:refs/heads/*"}, PushOptions{Remote: "mirror.name", Mirror: true}))
			for _, destinationPath := range destinationPaths {
				destinationCommitID, err := GetBranchCommitID(t.Context(), mockRepository(destinationPath), "master")
				require.NoError(t, err)
				assert.Equal(t, sourceCommitID, destinationCommitID)
				refs, _, err := gitcmd.NewCommand("for-each-ref", "--format=%(refname)").WithDir(destinationPath).RunStdString(t.Context())
				require.NoError(t, err)
				assert.Equal(t, "refs/heads/branch1\nrefs/heads/branch2\nrefs/heads/master\n", refs, "only the given refspecs are pushed")
			}
		})
	}

	assert.Equal(t, []string{
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_VALUE_0=c",
		"GIT_CONFIG_KEY_1=remote.x.url", "GIT_CONFIG_VALUE_1=u", "GIT_CONFIG_COUNT=2",
	}, appendConfigEnv([]string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_VALUE_0=c"}, configEntry{"remote.x.url", "u"}))
}
