// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package tempdir

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTempDir(t *testing.T) {
	base := t.TempDir()
	td := New(base, "sub")
	assert.Equal(t, filepath.Join(base, "sub"), td.JoinPath(""))

	// TODO: add some tests
}
