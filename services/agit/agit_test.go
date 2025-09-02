// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAgitPushOptionValue(t *testing.T) {
	assert.Equal(t, "a", parseAgitPushOptionValue("a"))
	assert.Equal(t, "a", parseAgitPushOptionValue("{base64}YQ=="))
	assert.Equal(t, "{base64}invalid value", parseAgitPushOptionValue("{base64}invalid value"))
}
