// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunWithContextStd(t *testing.T) {
	cmd := NewCommand(t.Context(), "--version")
	stdout, stderr, err := cmd.RunStdString(&RunOpts{})
	assert.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "git version")

	cmd = NewCommand(t.Context(), "--no-such-arg")
	stdout, stderr, err = cmd.RunStdString(&RunOpts{})
	if assert.Error(t, err) {
		assert.Equal(t, stderr, err.Stderr())
		assert.Contains(t, err.Stderr(), "unknown option:")
		assert.Contains(t, err.Error(), "exit status 129 - unknown option:")
		assert.Empty(t, stdout)
	}

	cmd = NewCommand(t.Context())
	cmd.AddDynamicArguments("-test")
	assert.ErrorIs(t, cmd.Run(&RunOpts{}), ErrBrokenCommand)

	cmd = NewCommand(t.Context())
	cmd.AddDynamicArguments("--test")
	assert.ErrorIs(t, cmd.Run(&RunOpts{}), ErrBrokenCommand)

	subCmd := "version"
	cmd = NewCommand(t.Context()).AddDynamicArguments(subCmd) // for test purpose only, the sub-command should never be dynamic for production
	stdout, stderr, err = cmd.RunStdString(&RunOpts{})
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
	cmd := NewCommandContextNoGlobals(t.Context(), "a", "-m msg", "it's a test", `say "hello"`)
	assert.EqualValues(t, cmd.prog+` a "-m msg" "it's a test" "say \"hello\""`, cmd.LogString())

	cmd = NewCommandContextNoGlobals(t.Context(), "url: https://a:b@c/", "/root/dir-a/dir-b")
	assert.EqualValues(t, cmd.prog+` "url: https://sanitized-credential@c/" .../dir-a/dir-b`, cmd.LogString())
}
