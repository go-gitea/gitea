// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package testlog

import (
	"testing"

	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
)

func TestLogChecker(t *testing.T) {
	_ = log.NewLogger(1000, "console", "console", `{"level":"info","stacktracelevel":"NONE","stderr":true}`)

	lc, cleanup := NewLogChecker(log.DEFAULT)
	defer cleanup()

	lc.ExpectContains("First", "Third")

	log.Info("test")
	assert.Error(t, lc.Check())

	log.Info("First")
	assert.Error(t, lc.Check())

	log.Info("Second")
	assert.Error(t, lc.Check())

	log.Info("Third")
	assert.NoError(t, lc.Check())
}
