// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDebounce(t *testing.T) {
	c := 0
	d := Debounce(50 * time.Millisecond)
	d(func() { c++ })
	assert.EqualValues(t, 0, c)
	d(func() { c++ })
	d(func() { c++ })
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 1, c)
	d(func() { c++ })
	assert.EqualValues(t, 1, c)
	d(func() { c++ })
	d(func() { c++ })
	d(func() { c++ })
	time.Sleep(100 * time.Millisecond)
	assert.EqualValues(t, 2, c)
}
