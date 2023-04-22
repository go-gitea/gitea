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
	_ = log.NewLogger(1000, "console", "console", `{"level":"info","stacktracelevel":"NONE","stderr":true}`)

	lc, cleanup := NewLogChecker(log.DEFAULT)
	defer cleanup()

	lc.Filter("First", "Third").StopMark("End")
	log.Info("test")

	filtered, stopped := lc.Check(100 * time.Millisecond)
	assert.EqualValues(t, []bool{false, false}, filtered)
	assert.EqualValues(t, false, stopped)

	log.Info("First")
	filtered, stopped = lc.Check(100 * time.Millisecond)
	assert.EqualValues(t, []bool{true, false}, filtered)
	assert.EqualValues(t, false, stopped)

	log.Info("Second")
	filtered, stopped = lc.Check(100 * time.Millisecond)
	assert.EqualValues(t, []bool{true, false}, filtered)
	assert.EqualValues(t, false, stopped)

	log.Info("Third")
	filtered, stopped = lc.Check(100 * time.Millisecond)
	assert.EqualValues(t, []bool{true, true}, filtered)
	assert.EqualValues(t, false, stopped)

	log.Info("End")
	filtered, stopped = lc.Check(100 * time.Millisecond)
	assert.EqualValues(t, []bool{true, true}, filtered)
	assert.EqualValues(t, true, stopped)
}
