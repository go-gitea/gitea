// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package json

// Allow "encoding/json" import.
import (
	"bytes"
	"encoding/binary"
	"encoding/json" //nolint:depguard
	"io"

	jsoniter "github.com/json-iterator/go"
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
	Unmarshal(data []byte, v any) error
	NewEncoder(writer io.Writer) Encoder
	NewDecoder(reader io.Reader) Decoder
	Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error
}

var (
	// DefaultJSONHandler default json handler
	DefaultJSONHandler Interface = JSONiter{jsoniter.ConfigCompatibleWithStandardLibrary}

	_ Interface = StdJSON{}
	_ Interface = JSONiter{}
)

// StdJSON implements Interface via encoding/json
type StdJSON struct{}

// Marshal implements Interface
func (StdJSON) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal implements Interface
func (StdJSON) Unmarshal(data []byte, v any) error {
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
func (j JSONiter) Marshal(v any) ([]byte, error) {
	return j.API.Marshal(v)
}

// Unmarshal implements Interface
func (j JSONiter) Unmarshal(data []byte, v any) error {
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

// Marshal converts object as bytes
func Marshal(v any) ([]byte, error) {
	return DefaultJSONHandler.Marshal(v)
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

// UnmarshalHandleDoubleEncode - due to a bug in xorm (see https://gitea.com/xorm/xorm/pulls/1957) - it's
// possible that a Blob may be double encoded or gain an unwanted prefix of 0xff 0xfe.
func UnmarshalHandleDoubleEncode(bs []byte, v any) error {
	err := json.Unmarshal(bs, v)
	if err != nil {
		ok := true
		rs := []byte{}
		temp := make([]byte, 2)
		for _, rn := range string(bs) {
			if rn > 0xffff {
				ok = false
				break
			}
			binary.LittleEndian.PutUint16(temp, uint16(rn))
			rs = append(rs, temp...)
		}
		if ok {
			if len(rs) > 1 && rs[0] == 0xff && rs[1] == 0xfe {
				rs = rs[2:]
			}
			err = json.Unmarshal(rs, v)
		}
	}
	if err != nil && len(bs) > 2 && bs[0] == 0xff && bs[1] == 0xfe {
		err = json.Unmarshal(bs[2:], v)
	}
	return err
}
