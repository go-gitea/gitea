// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushToExternalAddress(t *testing.T) {
	src := mockRepository("repo1_bare")
	srcCommitID, err := GetBranchCommitID(t.Context(), src, "master")
	require.NoError(t, err)

	for _, supportConfigEnv := range []bool{true, false} {
		defer test.MockVariableValue(&DefaultFeatures().SupportConfigEnv, supportConfigEnv)()
		dstPath := t.TempDir()
		require.NoError(t, InitRepositoryLocal(t.Context(), dstPath, true, Sha1ObjectFormat.Name()))
		require.NoError(t, PushToExternalAddress(t.Context(), src, dstPath, []string{"+refs/heads/*:refs/heads/*"}, PushOptions{Mirror: true}))
		dstCommitID, err := GetBranchCommitID(t.Context(), mockRepository(dstPath), "master")
		require.NoError(t, err)
		assert.Equal(t, srcCommitID, dstCommitID, "SupportConfigEnv=%v", supportConfigEnv)
		refs, _, err := gitcmd.NewCommand("for-each-ref", "--format=%(refname)").WithDir(dstPath).RunStdString(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "refs/heads/branch1\nrefs/heads/branch2\nrefs/heads/master\n", refs, "only the given refspecs are pushed")
	}

	env := appendConfigEnv([]string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_VALUE_0=c"}, ConfigEntry{"remote.x.url", "u"})
	assert.Equal(t, []string{
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_VALUE_0=c",
		"GIT_CONFIG_KEY_1=remote.x.url", "GIT_CONFIG_VALUE_1=u", "GIT_CONFIG_COUNT=2",
	}, env)
}
