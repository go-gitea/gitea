// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package json

import (
	"bytes"
	"strings"
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

func TestDecodeWithLimit(t *testing.T) {
	var v map[string]string
	assert.NoError(t, DecodeWithLimit(strings.NewReader(`{"a":"b"}`), 9, &v))
	assert.Equal(t, map[string]string{"a": "b"}, v)

	assert.ErrorIs(t, DecodeWithLimit(strings.NewReader(`{"a":"bc"}`), 9, &v), ErrTooLarge)
	assert.Error(t, DecodeWithLimit(strings.NewReader(`{"a":`), 9, &v))
	assert.NotErrorIs(t, DecodeWithLimit(strings.NewReader(`{"a":`), 9, &v), ErrTooLarge)
}
