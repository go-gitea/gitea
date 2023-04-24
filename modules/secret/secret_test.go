// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	var hex string
	var str string

	hex, _ = EncryptSecret("foo", "baz")
	str, _ = DecryptSecret("foo", hex)
	assert.Equal(t, "baz", str)

	hex, _ = EncryptSecret("bar", "baz")
	str, _ = DecryptSecret("foo", hex)
	assert.NotEqual(t, "baz", str)
}
