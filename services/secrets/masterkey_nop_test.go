// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNopMasterKey_IsSealed(t *testing.T) {
	k := NewNopMasterKeyProvider()
	assert.False(t, k.IsSealed())
}
