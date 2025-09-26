// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build goexperiment.jsonv2

package json

import (
	"bytes"
	jsonv1 "encoding/json"    //nolint:depguard // this package wraps it
	jsonv2 "encoding/json/v2" //nolint:depguard // this package wraps it
	"io"
)

// JSONv2 implements Interface via encoding/json/v2
// Requires GOEXPERIMENT=jsonv2 to be set at build time
type JSONv2 struct {
	marshalOptions                  jsonv2.Options
	marshalKeepOptionalEmptyOptions jsonv2.Options
	unmarshalOptions                jsonv2.Options
}

var jsonV2 JSONv2

func init() {
	commonMarshalOptions := []jsonv2.Options{
		jsonv2.FormatNilSliceAsNull(true),
		jsonv2.FormatNilMapAsNull(true),
	}
	jsonV2.marshalOptions = jsonv2.JoinOptions(commonMarshalOptions...)

	// Some JSON structs like oci.ImageConfig uses case-insensitive matching
	jsonV2.unmarshalOptions = jsonv2.JoinOptions(jsonv2.MatchCaseInsensitiveNames(true))

	// by default, "json/v2" omitempty removes all `""` empty strings, no matter where it comes from.
	// v1 has a different behavior: if the `""` is from a null pointer, or a Marshal function, it is kept.
	jsonV2.marshalKeepOptionalEmptyOptions = jsonv2.JoinOptions(append(commonMarshalOptions, jsonv1.OmitEmptyWithLegacySemantics(true))...)
}

func getDefaultJSONHandler() Interface {
	return &jsonV2
}

func MarshalKeepOptionalEmpty(v any) ([]byte, error) {
	return jsonv2.Marshal(v, jsonV2.marshalKeepOptionalEmptyOptions)
}

func (j JSONv2) Marshal(v any) ([]byte, error) {
	return jsonv2.Marshal(v, j.marshalOptions)
}

func (j JSONv2) Unmarshal(data []byte, v any) error {
	return jsonv2.Unmarshal(data, v, j.unmarshalOptions)
}

func (j JSONv2) NewEncoder(writer io.Writer) Encoder {
	return &encoderV2{writer: writer, opts: j.marshalOptions}
}

func (j JSONv2) NewDecoder(reader io.Reader) Decoder {
	return &decoderV2{reader: reader, opts: j.unmarshalOptions}
}

// Indent implements Interface using standard library (JSON v2 doesn't have Indent yet)
func (JSONv2) Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return jsonv1.Indent(dst, src, prefix, indent)
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
