// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_StatusTable(t *testing.T) {
	table := NewStatusTable()

	assert.False(t, table.IsRunning("xyz"))

	table.Start("xyz")
	assert.True(t, table.IsRunning("xyz"))

	assert.False(t, table.StartIfNotRunning("xyz"))
	assert.True(t, table.IsRunning("xyz"))

	table.Stop("xyz")
	assert.False(t, table.IsRunning("xyz"))

	assert.True(t, table.StartIfNotRunning("xyz"))
	assert.True(t, table.IsRunning("xyz"))

	table.Stop("xyz")
	assert.False(t, table.IsRunning("xyz"))
}
