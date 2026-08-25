// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDebounce(t *testing.T) {
	var c atomic.Int64
	d := Debounce(50 * time.Millisecond)
	d(func() { c.Add(1) })
	assert.EqualValues(t, 0, c.Load())
	d(func() { c.Add(1) })
	d(func() { c.Add(1) })
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 1, c.Load())
	d(func() { c.Add(1) })
	assert.EqualValues(t, 1, c.Load())
	d(func() { c.Add(1) })
	d(func() { c.Add(1) })
	d(func() { c.Add(1) })
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 2, c.Load())
}
