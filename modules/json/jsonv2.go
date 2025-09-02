// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package json

import (
	jsonv2 "encoding/json/v2"
	"io"
)

// isJSONv2Available returns true when JSON v2 is available (compiled with GOEXPERIMENT=jsonv2)
func isJSONv2Available() bool {
	return true
}

// marshalV2 uses JSON v2 marshal with v1 compatibility options
func marshalV2(v any) ([]byte, error) {
	opts := jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
		jsonv2.FormatNilSliceAsNull(true),
		jsonv2.FormatNilMapAsNull(true),
	)
	return jsonv2.Marshal(v, opts)
}

// unmarshalV2 uses JSON v2 unmarshal with v1 compatibility options
func unmarshalV2(data []byte, v any) error {
	opts := jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
	)
	return jsonv2.Unmarshal(data, v, opts)
}

// encoderV2 wraps JSON v2 streaming encoder
type encoderV2 struct {
	writer io.Writer
	opts   jsonv2.Options
}

func (e *encoderV2) Encode(v any) error {
	return jsonv2.MarshalWrite(e.writer, v, e.opts)
}

// newEncoderV2 creates a new JSON v2 streaming encoder
func newEncoderV2(writer io.Writer) Encoder {
	opts := jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
		jsonv2.FormatNilSliceAsNull(true),
		jsonv2.FormatNilMapAsNull(true),
	)
	return &encoderV2{writer: writer, opts: opts}
}

// decoderV2 wraps JSON v2 streaming decoder
type decoderV2 struct {
	reader io.Reader
	opts   jsonv2.Options
}

func (d *decoderV2) Decode(v any) error {
	return jsonv2.UnmarshalRead(d.reader, v, d.opts)
}

// newDecoderV2 creates a new JSON v2 streaming decoder
func newDecoderV2(reader io.Reader) Decoder {
	opts := jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
	)
	return &decoderV2{reader: reader, opts: opts}
}
