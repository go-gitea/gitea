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

func TestMarshalWriteAPI(t *testing.T) {
	v := struct {
		S []int          `json:"s"`
		M map[string]int `json:"m"`
		P *int           `json:"p"`
		U string         `json:"u"`
	}{U: "\xff"}
	buf := &bytes.Buffer{}
	assert.NoError(t, MarshalWriteAPI(buf, v))
	assert.JSONEq(t, `{"s":[],"m":{},"p":null,"u":"\ufffd"}`, buf.String())

	v.U = ""
	bs, err := Marshal(v)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"s":null,"m":null,"p":null,"u":""}`, string(bs))
}
