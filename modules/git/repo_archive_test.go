// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArchiveType(t *testing.T) {
	name, archiveType := SplitArchiveNameType("test.tar.gz")
	assert.Equal(t, "test", name)
	assert.Equal(t, "tar.gz", archiveType.String())

	name, archiveType = SplitArchiveNameType("a/b/test.zip")
	assert.Equal(t, "a/b/test", name)
	assert.Equal(t, "zip", archiveType.String())

	name, archiveType = SplitArchiveNameType("1234.bundle")
	assert.Equal(t, "1234", name)
	assert.Equal(t, "bundle", archiveType.String())

	name, archiveType = SplitArchiveNameType("test")
	assert.Equal(t, "test", name)
	assert.Equal(t, "unknown", archiveType.String())

	name, archiveType = SplitArchiveNameType("test.xz")
	assert.Equal(t, "test.xz", name)
	assert.Equal(t, "unknown", archiveType.String())
}
