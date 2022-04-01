// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
}
