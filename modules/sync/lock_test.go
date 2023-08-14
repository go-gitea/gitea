// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Lock(t *testing.T) {
	locker := GetLock("test")
	assert.NoError(t, locker.Lock())
	locker.Unlock()

	locked1, err1 := locker.TryLock()
	assert.NoError(t, err1)
	assert.True(t, locked1)

	locked2, err2 := locker.TryLock()
	assert.NoError(t, err2)
	assert.False(t, locked2)

	locker.Unlock()
}
