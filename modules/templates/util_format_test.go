// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountFmt(t *testing.T) {
	assert.Equal(t, "125", countFmt(125))
	assert.Equal(t, "1.3k", countFmt(int64(1317)))
	assert.Equal(t, "21.3M", countFmt(21317675))
	assert.Equal(t, "45.7G", countFmt(45721317675))
	assert.Equal(t, "", countFmt("test"))
}
