// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanUploadFileName(t *testing.T) {
	name := "this/is/test"
	cleanName := CleanUploadFileName(name)
	expectedCleanName := name
	assert.EqualValues(t, expectedCleanName, cleanName)

	name = "this/is/test/.git"
	cleanName = CleanUploadFileName(name)
	expectedCleanName = ""
	assert.EqualValues(t, expectedCleanName, cleanName)
}
