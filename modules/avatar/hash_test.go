// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar_test

import (
	"testing"

	"gitea.dev/modules/avatar"

	"github.com/stretchr/testify/assert"
)

func Test_HashAvatar(t *testing.T) {
	data := []byte("data")

	assert.Equal(t, "912b5d144e1f82719c79d4014fd9dc075aea506be410fb782d50c4f299f845f4", avatar.HashAvatar(1, data))
	assert.Equal(t, "55e41725707388f35483f881b2f38405fd9a6883365a862b8a5350ed6737c6fe", avatar.HashAvatar(8, data))
	assert.Equal(t, "473f06c83edcf69a9122db1d32f7649ebaa2680678e20ed0f868d25bd272abe8", avatar.HashAvatar(1024, data))
	assert.Equal(t, "161178642c7d59eb25a61dddced5e6b66eae1c70880d5f148b1b497b767e72d9", avatar.HashAvatar(1024, []byte{}))
}
