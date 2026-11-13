// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOnceValue(t *testing.T) {
	for _, name := range []string{"RepeatCall", "Panic"} {
		t.Run(name, func(t *testing.T) {
			callCount := 0
			doPanic := name == "Panic"
			o := OnceValue[int]{Func: func() int {
				callCount++
				if doPanic {
					panic("some error")
				}
				return 42
			}}
			if doPanic {
				assert.PanicsWithValue(t, "some error", func() { o.Value() })
				assert.PanicsWithValue(t, "some error", func() { o.Value() })
			} else {
				assert.Equal(t, 42, o.Value())
				assert.Equal(t, 42, o.Value())
			}
			assert.Equal(t, 1, callCount)
			doPanic = false
			o.Reset()
			assert.Equal(t, 42, o.Value())
			assert.Equal(t, 2, callCount)
			assert.Equal(t, 42, o.Value())
			assert.Equal(t, 2, callCount)
		})
	}
}
