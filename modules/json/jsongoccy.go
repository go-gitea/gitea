// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package json

import (
	"bytes"
	"io"

	"github.com/goccy/go-json"
)

var _ Interface = jsonGoccy{}

type jsonGoccy struct{}

func (jsonGoccy) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonGoccy) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonGoccy) NewEncoder(writer io.Writer) Encoder {
	return json.NewEncoder(writer)
}

func (jsonGoccy) NewDecoder(reader io.Reader) Decoder {
	return json.NewDecoder(reader)
}

func (jsonGoccy) Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
}
