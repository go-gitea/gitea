// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNopMasterKey_IsSealed(t *testing.T) {
	k := NewNopMasterKeyProvider()
	assert.False(t, k.IsSealed())
}
