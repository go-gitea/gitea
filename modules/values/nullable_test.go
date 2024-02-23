// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package values

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNullable(t *testing.T) {
	boolV := Nullable(true)
	assert.True(t, boolV.Value())
	assert.True(t, boolV.IsSome())
	assert.False(t, boolV.IsNone())

	nilBool := None[bool]()
	assert.False(t, nilBool.IsSome())
	assert.True(t, nilBool.IsNone())
}
