// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookMatrix(t *testing.T) {
	assert.Equal(t, "!roomid:domain", matrixRoomIDEncode("!roomid:domain"))
	assert.Equal(t, "!room%23id:domain", matrixRoomIDEncode("!room#id:domain")) // maybe it should never really happen in real world
}
