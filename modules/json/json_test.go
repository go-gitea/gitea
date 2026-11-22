// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package json

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndent(t *testing.T) {
	buf := &bytes.Buffer{}
	err := Indent(buf, []byte(`{"a":1}`), ">", "  ")
	assert.NoError(t, err)
	assert.Equal(t, `{
>  "a": 1
>}`, buf.String())
}
