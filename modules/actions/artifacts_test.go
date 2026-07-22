// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildSignature(t *testing.T) {
	a := BuildSignature("v0", "x")
	b := BuildSignature("v0", "x")
	assert.Equal(t, a, b)

	a = BuildSignature("v0", "x", "yz")
	b = BuildSignature("v0", "xy", "z")
	assert.NotEqual(t, a, b)

	a = BuildSignature("v1", "x")
	b = BuildSignature("v2", "x")
	assert.NotEqual(t, a, b)

	a = BuildSignature("v0", "x")
	b = BuildSignature("v0x")
	assert.NotEqual(t, a, b)

	a = BuildSignature("v0", "", "x")
	b = BuildSignature("v0", "x", "")
	assert.NotEqual(t, a, b)

	a = BuildSignature("v0")
	b = BuildSignature("v0")
	assert.Equal(t, a, b)
}
