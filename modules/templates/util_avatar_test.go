// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAvatarHTML(t *testing.T) {
	assert.Equal(t, `<img loading="lazy" alt class="avatar" src="/a.png" title="A &amp; B" width="20" height="20">`, string(AvatarHTML("/a.png", 20, "avatar", "A & B")))
	assert.Equal(t, `<img loading="lazy" alt class="avatar" src="/a.png" width="20" height="20">`, string(AvatarHTML("/a.png", 20, "avatar", "")))
}
