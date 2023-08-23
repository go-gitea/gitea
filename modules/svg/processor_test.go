// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package svg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalize(t *testing.T) {
	res := Normalize([]byte("foo"), 1)
	assert.Equal(t, "foo", string(res))

	res = Normalize([]byte(`<?xml version="1.0"?>
<!--
comment
-->
<svg xmlns = "...">content</svg>`), 1)
	assert.Equal(t, `<svg width="1" height="1" class="svg">content</svg>`, string(res))

	res = Normalize([]byte(`<svg
width="100"
class="svg-icon"
>content</svg>`), 16)

	assert.Equal(t, `<svg class="svg-icon" width="16" height="16">content</svg>`, string(res))
}
