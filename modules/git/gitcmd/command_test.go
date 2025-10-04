// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"fmt"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/tempdir"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	gitHomePath, cleanup, err := tempdir.OsTempDir("gitea-test").MkdirTempRandom("git-home")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to create temp dir: %v", err)
		os.Exit(1)
	}
	defer cleanup()

	setting.Git.HomePath = gitHomePath
}

func TestRunWithContextStd(t *testing.T) {
	cmd := NewCommand("--version")
	stdout, stderr, err := cmd.RunStdString(t.Context())
	assert.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "git version")

	cmd = NewCommand("--no-such-arg")
	stdout, stderr, err = cmd.RunStdString(t.Context())
	if assert.Error(t, err) {
		assert.Contains(t, stderr, "unknown option:")
		assert.Contains(t, err.Error(), "exit status 129")
		assert.Empty(t, stdout)
	}

	cmd = NewCommand()
	cmd.AddDynamicArguments("-test")
	assert.ErrorIs(t, cmd.Run(t.Context()), ErrBrokenCommand)

	cmd = NewCommand()
	cmd.AddDynamicArguments("--test")
	assert.ErrorIs(t, cmd.Run(t.Context()), ErrBrokenCommand)

	subCmd := "version"
	cmd = NewCommand().AddDynamicArguments(subCmd) // for test purpose only, the sub-command should never be dynamic for production
	stdout, stderr, err = cmd.RunStdString(t.Context())
	assert.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "git version")
}

func TestGitArgument(t *testing.T) {
	assert.True(t, isValidArgumentOption("-x"))
	assert.True(t, isValidArgumentOption("--xx"))
	assert.False(t, isValidArgumentOption(""))
	assert.False(t, isValidArgumentOption("x"))

	assert.True(t, isSafeArgumentValue(""))
	assert.True(t, isSafeArgumentValue("x"))
	assert.False(t, isSafeArgumentValue("-x"))
}

func TestCommandString(t *testing.T) {
	cmd := NewCommand("a", "-m msg", "it's a test", `say "hello"`)
	assert.Equal(t, cmd.prog+` a "-m msg" "it's a test" "say \"hello\""`, cmd.LogString())

	cmd = NewCommand("url: https://a:b@c/", "/root/dir-a/dir-b")
	assert.Equal(t, cmd.prog+` "url: https://sanitized-credential@c/" .../dir-a/dir-b`, cmd.LogString())
}
