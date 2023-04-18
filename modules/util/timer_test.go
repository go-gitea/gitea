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
	var c int64
	d := Debounce(50 * time.Millisecond)
	d(func() { atomic.AddInt64(&c, 1) })
	assert.EqualValues(t, 0, atomic.LoadInt64(&c))
	d(func() { atomic.AddInt64(&c, 1) })
	d(func() { atomic.AddInt64(&c, 1) })
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 1, atomic.LoadInt64(&c))
	d(func() { atomic.AddInt64(&c, 1) })
	assert.EqualValues(t, 1, atomic.LoadInt64(&c))
	d(func() { atomic.AddInt64(&c, 1) })
	d(func() { atomic.AddInt64(&c, 1) })
	d(func() { atomic.AddInt64(&c, 1) })
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 2, atomic.LoadInt64(&c))
}
