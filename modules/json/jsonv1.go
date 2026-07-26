// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package json

import (
	"bytes"
	"encoding/json" //nolint:depguard // this package wraps it
	"io"
)

type jsonV1 struct{}

var _ Interface = jsonV1{}

func (jsonV1) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonV1) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonV1) NewEncoder(writer io.Writer) Encoder {
	return json.NewEncoder(writer)
}

func (jsonV1) NewDecoder(reader io.Reader) Decoder {
	return json.NewDecoder(reader)
}

func (jsonV1) Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
}
