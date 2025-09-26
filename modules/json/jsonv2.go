// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build goexperiment.jsonv2

package json

import (
	"bytes"
	jsonv2 "encoding/json/v2" //nolint:depguard // this package wraps it
	"io"
)

// JSONv2 implements Interface via encoding/json/v2
// Requires GOEXPERIMENT=jsonv2 to be set at build time
type JSONv2 struct{}

var _ Interface = JSONv2{}

func getDefaultJSONHandler() Interface {
	return JSONv2{}
}

func jsonv2DefaultMarshalOptions() jsonv2.Options {
	return jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
		jsonv2.FormatNilSliceAsNull(true),
		jsonv2.FormatNilMapAsNull(true),
		jsonv2.Deterministic(true),
	)
}

func jsonv2DefaultUnmarshalOptions() jsonv2.Options {
	return jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
	)
}

func (JSONv2) Marshal(v any) ([]byte, error) {
	return jsonv2.Marshal(v, jsonv2DefaultMarshalOptions())
}

func (JSONv2) Unmarshal(data []byte, v any) error {
	// legacy behavior: treat empty or whitespace-only input as no input, it should be safe
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	return jsonv2.Unmarshal(data, v, jsonv2DefaultUnmarshalOptions())
}

func (JSONv2) NewEncoder(writer io.Writer) Encoder {
	return &encoderV2{writer: writer, opts: jsonv2DefaultMarshalOptions()}
}

func (JSONv2) NewDecoder(reader io.Reader) Decoder {
	return &decoderV2{reader: reader, opts: jsonv2DefaultMarshalOptions()}
}

// Indent implements Interface using standard library (JSON v2 doesn't have Indent yet)
func (JSONv2) Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
}

type encoderV2 struct {
	writer io.Writer
	opts   jsonv2.Options
}

func (e *encoderV2) Encode(v any) error {
	return jsonv2.MarshalWrite(e.writer, v, e.opts)
}

type decoderV2 struct {
	reader io.Reader
	opts   jsonv2.Options
}

func (d *decoderV2) Decode(v any) error {
	return jsonv2.UnmarshalRead(d.reader, v, d.opts)
}
