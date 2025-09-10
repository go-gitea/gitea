// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build goexperiment.jsonv2

package json

import (
	"bytes"
	jsonv2 "encoding/json/v2" //nolint:depguard // this package wraps it
	"io"
)

// isJSONv2Available returns true when JSON v2 is available (compiled with GOEXPERIMENT=jsonv2)
func isJSONv2Available() bool {
	return true
}

// marshalV2Internal uses JSON v2 marshal with v1 compatibility options (no trailing newline)
func marshalV2Internal(v any) ([]byte, error) {
	opts := jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
		jsonv2.FormatNilSliceAsNull(true),
		jsonv2.FormatNilMapAsNull(true),
		jsonv2.Deterministic(true),
	)
	return jsonv2.Marshal(v, opts)
}

// marshalV2 uses JSON v2 marshal with v1 compatibility options (with trailing newline for compatibility with standard library)
func marshalV2(v any) ([]byte, error) {
	result, err := marshalV2Internal(v)
	if err != nil {
		return nil, err
	}

	return append(result, '\n'), nil
}

// unmarshalV2 uses JSON v2 unmarshal with v1 compatibility options
func unmarshalV2(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}

	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}

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
	err := jsonv2.MarshalWrite(e.writer, v, e.opts)
	if err != nil {
		return err
	}

	_, err = e.writer.Write([]byte{'\n'})
	return err
}

// newEncoderV2 creates a new JSON v2 streaming encoder
func newEncoderV2(writer io.Writer) Encoder {
	opts := jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
		jsonv2.FormatNilSliceAsNull(true),
		jsonv2.FormatNilMapAsNull(true),
		jsonv2.Deterministic(true),
	)
	return &encoderV2{writer: writer, opts: opts}
}

// decoderV2 wraps JSON v2 streaming decoder
type decoderV2 struct {
	reader io.Reader
	opts   jsonv2.Options
}

func (d *decoderV2) Decode(v any) error {
	err := jsonv2.UnmarshalRead(d.reader, v, d.opts)
	// Handle EOF more gracefully to match standard library behavior
	if err != nil && err.Error() == "unexpected EOF" {
		return io.EOF
	}
	return err
}

// newDecoderV2 creates a new JSON v2 streaming decoder
func newDecoderV2(reader io.Reader) Decoder {
	opts := jsonv2.JoinOptions(
		jsonv2.MatchCaseInsensitiveNames(true),
	)
	return &decoderV2{reader: reader, opts: opts}
}
