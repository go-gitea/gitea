// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package json

import (
	"bytes"
	"encoding/json" //nolint:depguard // this package wraps it
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
	MarshalWrite(w io.Writer, v any) error
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

// MarshalWrite writes the JSON encoding of v to the given writer
func MarshalWrite(w io.Writer, v any) error {
	return DefaultJSONHandler.MarshalWrite(w, v)
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
