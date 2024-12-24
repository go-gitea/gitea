// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallerFuncName(t *testing.T) {
	s := CallerFuncName(1)
	assert.Equal(t, "code.gitea.io/gitea/modules/util.TestCallerFuncName", s)
}

func BenchmarkCallerFuncName(b *testing.B) {
	// BenchmarkCaller/sprintf-12         	12744829	        95.49 ns/op
	b.Run("sprintf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = fmt.Sprintf("aaaaaaaaaaaaaaaa %s %s %s", "bbbbbbbbbbbbbbbbbbb", b.Name(), "ccccccccccccccccccccc")
		}
	})
	// BenchmarkCaller/caller-12          	10625133	       113.6 ns/op
	// It is almost as fast as fmt.Sprintf
	b.Run("caller", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			CallerFuncName(1)
		}
	})
}
