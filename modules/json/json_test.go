// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package json

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGiteaDBJSONUnmarshal(t *testing.T) {
	var m map[any]any
	err := UnmarshalHandleDoubleEncode(nil, &m)
	assert.NoError(t, err)
	err = UnmarshalHandleDoubleEncode([]byte(""), &m)
	assert.NoError(t, err)
}
