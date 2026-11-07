// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRemoteNotExistError(t *testing.T) {
	_, err := GetRemoteAddress(t.Context(), mockRepository("repo1_bare"), "no-such-remote")
	assert.True(t, IsRemoteNotExistError(err))
}
