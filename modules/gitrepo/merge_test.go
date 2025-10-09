// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseMergeTreeOutput(t *testing.T) {
	conflictedOutput := "837480c2773160381cbe6bcce90f7732789b5856\x00options/locale/locale_en-US.ini\x00services/webhook/webhook_test.go\x00"
	treeID, conflictedFiles, err := parseMergeTreeOutput(conflictedOutput)
	assert.NoError(t, err)
	assert.Equal(t, "837480c2773160381cbe6bcce90f7732789b5856", treeID)
	assert.Len(t, conflictedFiles, 2)
	assert.Equal(t, "options/locale/locale_en-US.ini", conflictedFiles[0])
	assert.Equal(t, "services/webhook/webhook_test.go", conflictedFiles[1])

	nonConflictedOutput := "837480c2773160381cbe6bcce90f7732789b5856\x00"
	treeID, conflictedFiles, err = parseMergeTreeOutput(nonConflictedOutput)
	assert.NoError(t, err)
	assert.Equal(t, "837480c2773160381cbe6bcce90f7732789b5856", treeID)
	assert.Empty(t, conflictedFiles)
}
