// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	goccy_json "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
)

// Encoder represents an encoder for json
type Encoder interface {
	Encode(v interface{}) error
}

// Decoder represents a decoder for json
type Decoder interface {
	Decode(v interface{}) error
}

// Interface represents an interface to handle json data
type Interface interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
	NewEncoder(writer io.Writer) Encoder
	NewDecoder(reader io.Reader) Decoder
	Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error
}

// DefaultJSONHandlerType is the type of library used as the backend for the JSON library
var DefaultJSONHandlerType = "jsoniter"

var (
	// DefaultJSONHandler default json handler
	DefaultJSONHandler Interface = JSONiter{}

	_ Interface = StdJSON{}
	_ Interface = JSONiter{}
	_ Interface = GoCcyJSON{}
)

func init() {
	SelectDefaultJSONHandler(DefaultJSONHandlerType)
}

// SelectDefaultJSONHandler selects the default JSON handler
// Note: this function is not race safe and would need to be run before other uses
func SelectDefaultJSONHandler(typ string) {
	if typ == DefaultJSONHandlerType {
		return
	}
	switch strings.ToLower(typ) {
	case "std":
		DefaultJSONHandler = StdJSON{}
		DefaultJSONHandlerType = "std"
	case "goccy":
		DefaultJSONHandler = GoCcyJSON{}
		DefaultJSONHandlerType = "goccy"
	case "jsoniter":
		fallthrough
	default:
		DefaultJSONHandler = JSONiter{jsoniter.ConfigCompatibleWithStandardLibrary}
		DefaultJSONHandlerType = "jsoniter"
	}
}

// StdJSON implements Interface via encoding/json
type StdJSON struct{}

// Marshal implements Interface
func (StdJSON) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal implements Interface
func (StdJSON) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// NewEncoder implements Interface
func (StdJSON) NewEncoder(writer io.Writer) Encoder {
	return json.NewEncoder(writer)
}

// NewDecoder implements Interface
func (StdJSON) NewDecoder(reader io.Reader) Decoder {
	return json.NewDecoder(reader)
}

// Indent implements Interface
func (StdJSON) Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
}

// JSONiter implements Interface via jsoniter
type JSONiter struct {
	jsoniter.API
}

// Marshal implements Interface
func (j JSONiter) Marshal(v interface{}) ([]byte, error) {
	return j.API.Marshal(v)
}

// Unmarshal implements Interface
func (j JSONiter) Unmarshal(data []byte, v interface{}) error {
	return j.API.Unmarshal(data, v)
}

// NewEncoder implements Interface
func (j JSONiter) NewEncoder(writer io.Writer) Encoder {
	return j.API.NewEncoder(writer)
}

// NewDecoder implements Interface
func (j JSONiter) NewDecoder(reader io.Reader) Decoder {
	return j.API.NewDecoder(reader)
}

// Indent implements Interface, since jsoniter don't support Indent, just use encoding/json's
func (j JSONiter) Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
}

// GoCcyJSON implements Interface
type GoCcyJSON struct{}

// Marshal implements Interface
func (j GoCcyJSON) Marshal(v interface{}) ([]byte, error) {
	return goccy_json.Marshal(v)
}

// Unmarshal implements Interface
func (j GoCcyJSON) Unmarshal(data []byte, v interface{}) error {
	return goccy_json.Unmarshal(data, v)
}

// NewEncoder implements Interface
func (j GoCcyJSON) NewEncoder(writer io.Writer) Encoder {
	return goccy_json.NewEncoder(writer)
}

// NewDecoder implements Interface
func (j GoCcyJSON) NewDecoder(reader io.Reader) Decoder {
	return goccy_json.NewDecoder(reader)
}

// Indent implements Interface
func (j GoCcyJSON) Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return goccy_json.Indent(dst, src, prefix, indent)
}

// Marshal converts object as bytes
func Marshal(v interface{}) ([]byte, error) {
	return DefaultJSONHandler.Marshal(v)
}

// Unmarshal decodes object from bytes
func Unmarshal(data []byte, v interface{}) error {
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
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
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
