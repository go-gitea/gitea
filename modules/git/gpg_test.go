// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalCommitSignSettings(t *testing.T) {
	defer test.MockVariableValue(&setting.Git.HomePath)()
	defer GlobalCommitSignSettings.Reset()

	signWithGitConfig := func(gitConfig string) bool {
		setting.Git.HomePath = t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(setting.Git.HomePath, ".gitconfig"), []byte(gitConfig), 0o600))
		GlobalCommitSignSettings.Reset()
		return GlobalCommitSignSettings.Value().Sign
	}

	assert.False(t, signWithGitConfig("[user]\n\tsigningkey = KEY\n"), "must not sign when commit.gpgsign is unset")
	assert.True(t, signWithGitConfig("[user]\n\tsigningkey = KEY\n[commit]\n\tgpgsign = true\n"))
	assert.True(t, signWithGitConfig("[user]\n\tsigningkey = KEY\n[commit]\n\tgpgsign\n"), "git reads a valueless commit.gpgsign as true")
	assert.False(t, signWithGitConfig("[user]\n\tsigningkey = KEY\n[commit]\n\tgpgsign = nonsense\n"), "must not sign when git cannot parse commit.gpgsign")
}
