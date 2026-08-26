// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar_test

import (
	"testing"

	"gitea.dev/modules/avatar"

	"github.com/stretchr/testify/assert"
)

func Test_HashAvatar(t *testing.T) {
	buf := []byte{1, 2}
	assert.Equal(t, "057b0e5aa7ef2504b886951f1355b3386130992bc867cb00c16babcc441571da", avatar.HashAvatar(1, buf))
	assert.Equal(t, "18a4a808c40d70ed532b6f19dfe7b8732eeb727e375c87a9944fde7555de285c", avatar.HashAvatar(8, buf))
	assert.Equal(t, "5265546b79c483b2c399bba1df5746f77bba7145bb9f53b7b9c6b0fb57dc25eb", avatar.HashAvatar(1024, buf))
	assert.Equal(t, "161178642c7d59eb25a61dddced5e6b66eae1c70880d5f148b1b497b767e72d9", avatar.HashAvatar(1024, []byte{}))
}
