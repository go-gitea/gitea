// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilenameIndexerID(t *testing.T) {
	assert.EqualValues(t, "9ix_r_test.txt", FilenameIndexerID(12345, false, "test.txt"))
	assert.EqualValues(t, "9ix_w_test.txt", FilenameIndexerID(12345, true, "test.txt"))
	assert.EqualValues(t, "n_r_you don't know how to name a file?", FilenameIndexerID(23, false, "you don't know how to name a file?"))
}

func TestParseIndexerID(t *testing.T) {
	repoID, isWiki, filename, err := ParseIndexerID("9ix_r_test.txt")
	assert.NoError(t, err)
	assert.EqualValues(t, 12345, repoID)
	assert.False(t, isWiki)
	assert.EqualValues(t, "test.txt", filename)

	_, _, _, err = ParseIndexerID("9ix_r")
	assert.Error(t, err)
}
