// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanUploadFileName(t *testing.T) {
	t.Run("Clean regular file", func(t *testing.T) {
		name := "this/is/test"
		cleanName := CleanUploadFileName(name)
		expectedCleanName := name
		assert.Equal(t, expectedCleanName, cleanName)
	})

	t.Run("Clean a .git path", func(t *testing.T) {
		name := "this/is/test/.git"
		cleanName := CleanUploadFileName(name)
		expectedCleanName := ""
		assert.Equal(t, expectedCleanName, cleanName)
	})
}
