// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/tempdir"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// FIXME: GIT-PACKAGE-DEPENDENCY: the dependency is not right.
	// "setting.Git.HomePath" is initialized in "git" package but really used in "gitcmd" package
	gitHomePath, cleanup, err := tempdir.OsTempDir("gitea-test").MkdirTempRandom("git-home")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to create temp dir: %v", err)
		os.Exit(1)
	}
	defer cleanup()

	setting.Git.HomePath = gitHomePath
	os.Exit(m.Run())
}

func TestRunWithContextStd(t *testing.T) {
	{
		cmd := NewCommand("--version")
		stdout, stderr, err := cmd.RunStdString(t.Context())
		assert.NoError(t, err)
		assert.Empty(t, stderr)
		assert.Contains(t, stdout, "git version")
	}

	{
		cmd := NewCommand("ls-tree", "no-such")
		stdout, stderr, err := cmd.RunStdString(t.Context())
		if assert.Error(t, err) {
			assert.Equal(t, stderr, err.Stderr())
			assert.Equal(t, "fatal: Not a valid object name no-such\n", err.Stderr())
			// FIXME: GIT-CMD-STDERR: it is a bad design, the stderr should not be put in the error message
			assert.Equal(t, "exit status 128 - fatal: Not a valid object name no-such", err.Error())
			assert.Empty(t, stdout)
		}
	}

	{
		cmd := NewCommand("ls-tree", "no-such")
		stdout, stderr, err := cmd.RunStdBytes(t.Context())
		if assert.Error(t, err) {
			assert.Equal(t, string(stderr), err.Stderr())
			assert.Equal(t, "fatal: Not a valid object name no-such\n", err.Stderr())
			// FIXME: GIT-CMD-STDERR: it is a bad design, the stderr should not be put in the error message
			assert.Equal(t, "exit status 128 - fatal: Not a valid object name no-such", err.Error())
			assert.Empty(t, stdout)
		}
	}

	{
		cmd := NewCommand()
		cmd.AddDynamicArguments("-test")
		assert.ErrorIs(t, cmd.Run(t.Context()), ErrBrokenCommand)

		cmd = NewCommand()
		cmd.AddDynamicArguments("--test")
		assert.ErrorIs(t, cmd.Run(t.Context()), ErrBrokenCommand)
	}

	{
		subCmd := "version"
		cmd := NewCommand().AddDynamicArguments(subCmd) // for test purpose only, the sub-command should never be dynamic for production
		stdout, stderr, err := cmd.RunStdString(t.Context())
		assert.NoError(t, err)
		assert.Empty(t, stderr)
		assert.Contains(t, stdout, "git version")
	}
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

func TestRunStdError(t *testing.T) {
	e := &runStdError{stderr: "some error"}
	var err RunStdError = e

	var asErr RunStdError
	require.ErrorAs(t, err, &asErr)
	require.Equal(t, "some error", asErr.Stderr())

	require.ErrorAs(t, fmt.Errorf("wrapped %w", err), &asErr)
}

func TestRunWithContextTimeout(t *testing.T) {
	t.Run("NoTimeout", func(t *testing.T) {
		// 'git --version' does not block so it must be finished before the timeout triggered.
		err := NewCommand("--version").Run(t.Context())
		require.NoError(t, err)
	})
	t.Run("WithTimeout", func(t *testing.T) {
		cmd := NewCommand("hash-object", "--stdin")
		_, _, pipeClose := cmd.MakeStdinStdoutPipe()
		defer pipeClose()
		err := cmd.WithTimeout(1 * time.Millisecond).Run(t.Context())
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
