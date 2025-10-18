// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_removeSSHKeyComment(t *testing.T) {
	content, err := removeSSHKeyComment("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINna5Jd6FTG4d87pUHnd/uLBr/6zGOVVFEQmdTs6k21L user@hostname")
	assert.NoError(t, err)
	assert.Equal(t, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINna5Jd6FTG4d87pUHnd/uLBr/6zGOVVFEQmdTs6k21L", content)
}
