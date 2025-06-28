// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashFilePathForWebUI(t *testing.T) {
	assert.Equal(t,
		"8843d7f92416211de9ebb963ff4ce28125932878",
		HashFilePathForWebUI("foobar"),
	)
}
