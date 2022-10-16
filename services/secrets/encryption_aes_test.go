// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	provider := NewAesEncryptionProvider()
	key := []byte("1111111111111111")
	pri := "vvvvvvv"
	enc, err := provider.EncryptString(pri, key)
	assert.NoError(t, err)
	v, err := provider.DecryptString(enc, key)
	assert.NoError(t, err)
	assert.EqualValues(t, pri, v)
}
