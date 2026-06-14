// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateDistributionOrComponent(t *testing.T) {
	bad := []string{
		"",
		".",
		"..",
		"-stable",
		".hidden",
		"a/b",
		"a b",
		"bookworm\nSigned-By: evil",
		"main\nFilename: pool/x",
		"a\tb",
	}
	for _, name := range bad {
		assert.False(t, isValidDistributionOrComponent(name), "bad=%q", name)
	}

	good := []string{
		"stable",
		"bookworm",
		"bookworm-backports",
		"stable-updates",
		"main",
		"non-free-firmware",
		"a",
		"1",
	}
	for _, name := range good {
		assert.True(t, isValidDistributionOrComponent(name), "good=%q", name)
	}
}
