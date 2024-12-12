// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type S1 struct {
	a, b, c int
}

func (s S1) S1Func() {}

type S1Intf interface {
	S1Func()
}

type S2 struct {
	a, b int
}

func TestPtr(t *testing.T) {
	s1 := &S1{1, 2, 3}
	var intf S1Intf = s1
	s2 := ptr[S2](intf)
	assert.Equal(t, 1, s2.a)
	assert.Equal(t, 2, s2.b)
}
