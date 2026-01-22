// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseMergeTreeOutput(t *testing.T) {
	conflictedOutput := "837480c2773160381cbe6bcce90f7732789b5856\x00options/locale/locale_en-US.ini\x00services/webhook/webhook_test.go\x00"
	treeID, conflictedFiles, err := parseMergeTreeOutput(strings.NewReader(conflictedOutput), 10)
	assert.NoError(t, err)
	assert.Equal(t, "837480c2773160381cbe6bcce90f7732789b5856", treeID)
	assert.Len(t, conflictedFiles, 2)
	assert.Equal(t, "options/locale/locale_en-US.ini", conflictedFiles[0])
	assert.Equal(t, "services/webhook/webhook_test.go", conflictedFiles[1])

	nonConflictedOutput := "837480c2773160381cbe6bcce90f7732789b5856\x00"
	treeID, conflictedFiles, err = parseMergeTreeOutput(strings.NewReader(nonConflictedOutput), 10)
	assert.NoError(t, err)
	assert.Equal(t, "837480c2773160381cbe6bcce90f7732789b5856", treeID)
	assert.Empty(t, conflictedFiles)
}
