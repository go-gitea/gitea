// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
)

func TestLogChecker(t *testing.T) {
	lc, cleanup := NewLogChecker(log.DEFAULT)
	defer cleanup()

	lc.Filter("First", "Third").StopMark("End")
	log.Info("test")

	filtered, stopped := lc.Check(100 * time.Millisecond)
	assert.ElementsMatch(t, []bool{false, false}, filtered)
	assert.False(t, stopped)

	log.Info("First")
	filtered, stopped = lc.Check(100 * time.Millisecond)
	assert.ElementsMatch(t, []bool{true, false}, filtered)
	assert.False(t, stopped)

	log.Info("Second")
	filtered, stopped = lc.Check(100 * time.Millisecond)
	assert.ElementsMatch(t, []bool{true, false}, filtered)
	assert.False(t, stopped)

	log.Info("Third")
	filtered, stopped = lc.Check(100 * time.Millisecond)
	assert.ElementsMatch(t, []bool{true, true}, filtered)
	assert.False(t, stopped)

	log.Info("End")
	filtered, stopped = lc.Check(100 * time.Millisecond)
	assert.ElementsMatch(t, []bool{true, true}, filtered)
	assert.True(t, stopped)
}
