// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package json

import (
	"bytes"
	"encoding/json" //nolint:depguard // this package wraps it
	"errors"
	"io"
)

// Encoder represents an encoder for json
type Encoder interface {
	Encode(v any) error
}

// Decoder represents a decoder for json
type Decoder interface {
	Decode(v any) error
}

// Interface represents an interface to handle json data
type Interface interface {
	Marshal(v any) ([]byte, error)
	MarshalWriteRelaxed(w io.Writer, v any) error
	Unmarshal(data []byte, v any) error
	NewEncoder(writer io.Writer) Encoder
	NewDecoder(reader io.Reader) Decoder
	Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error
}

var DefaultJSONHandler = getDefaultJSONHandler()

// Marshal converts object as bytes
func Marshal(v any) ([]byte, error) {
	return DefaultJSONHandler.Marshal(v)
}

// MarshalWriteRelaxed writes the JSON encoding of v to the given writer
// It tolerates invalid UTF-8 in strings and replaces them with the Unicode replacement character U+FFFD.
func MarshalWriteRelaxed(w io.Writer, v any) error {
	return DefaultJSONHandler.MarshalWriteRelaxed(w, v)
}

// Unmarshal decodes object from bytes
func Unmarshal(data []byte, v any) error {
	return DefaultJSONHandler.Unmarshal(data, v)
}

// NewEncoder creates an encoder to write objects to writer
func NewEncoder(writer io.Writer) Encoder {
	return DefaultJSONHandler.NewEncoder(writer)
}

// NewDecoder creates a decoder to read objects from reader
func NewDecoder(reader io.Reader) Decoder {
	return DefaultJSONHandler.NewDecoder(reader)
}

// ErrTooLarge is returned by DecodeWithLimit when the input is larger than the limit
var ErrTooLarge = errors.New("json: input exceeds the size limit")

// DecodeWithLimit decodes a JSON value from reader into v, reading at most limit bytes.
// It returns ErrTooLarge if the input is larger than that, so untrusted input can't use unbounded memory.
func DecodeWithLimit(reader io.Reader, limit int64, v any) error {
	lr := &io.LimitedReader{R: reader, N: limit + 1}
	err := NewDecoder(lr).Decode(v)
	if lr.N <= 0 {
		return ErrTooLarge
	}
	return err
}

// Indent appends to dst an indented form of the JSON-encoded src.
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return DefaultJSONHandler.Indent(dst, src, prefix, indent)
}

// MarshalIndent copied from encoding/json
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	b, err := Marshal(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = Indent(&buf, b, prefix, indent)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Valid proxy to json.Valid
func Valid(data []byte) bool {
	return json.Valid(data)
}
