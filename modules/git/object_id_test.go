// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidSHAPattern(t *testing.T) {
	h := Sha1ObjectFormat
	assert.True(t, h.IsValid("fee1"))
	assert.True(t, h.IsValid("abc000"))
	assert.True(t, h.IsValid("9023902390239023902390239023902390239023"))
	assert.False(t, h.IsValid("90239023902390239023902390239023902390239023"))
	assert.False(t, h.IsValid("abc"))
	assert.False(t, h.IsValid("123g"))
	assert.False(t, h.IsValid("some random text"))
}
