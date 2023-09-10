// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunWithContextStd(t *testing.T) {
	cmd := NewCommand(context.Background(), "--version")
	stdout, stderr, err := cmd.RunStdString(&RunOpts{})
	assert.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "git version")

	cmd = NewCommand(context.Background(), "--no-such-arg")
	stdout, stderr, err = cmd.RunStdString(&RunOpts{})
	if assert.Error(t, err) {
		assert.Equal(t, stderr, err.Stderr())
		assert.Contains(t, err.Stderr(), "unknown option:")
		assert.Contains(t, err.Error(), "exit status 129 - unknown option:")
		assert.Empty(t, stdout)
	}

	cmd = NewCommand(context.Background())
	cmd.AddDynamicArguments("-test")
	assert.ErrorIs(t, cmd.Run(&RunOpts{}), ErrBrokenCommand)

	cmd = NewCommand(context.Background())
	cmd.AddDynamicArguments("--test")
	assert.ErrorIs(t, cmd.Run(&RunOpts{}), ErrBrokenCommand)

	subCmd := "version"
	cmd = NewCommand(context.Background()).AddDynamicArguments(subCmd) // for test purpose only, the sub-command should never be dynamic for production
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
	cmd := NewCommandContextNoGlobals(context.Background(), "a", "-m msg", "it's a test", `say "hello"`)
	assert.EqualValues(t, cmd.prog+` a "-m msg" "it's a test" "say \"hello\""`, cmd.String())

	cmd = NewCommandContextNoGlobals(context.Background(), "url: https://a:b@c/")
	assert.EqualValues(t, cmd.prog+` "url: https://sanitized-credential@c/"`, cmd.toString(true))
}
