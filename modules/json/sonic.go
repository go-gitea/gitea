// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build exp_json
// +build exp_json

package json

// Allow "encoding/json" import.
import (
	"bytes"
	"encoding/binary"
	"encoding/json" //nolint:depguard
	"io"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"github.com/bytedance/sonic/encoder"
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

var (
	// DefaultJSONHandler default json handler
	DefaultJSONHandler Interface = Sonic{}

	_ Interface = StdJSON{}
	_ Interface = Sonic{}
)

// StdJSON implements Interface via encoding/json
type StdJSON struct{}

// Marshal implements Interface
func (StdJSON) Marshal(v interface{}) ([]byte, error) {
	return sonic.Marshal(v)
}

// Unmarshal implements Interface
func (StdJSON) Unmarshal(data []byte, v interface{}) error {
	return sonic.Unmarshal(data, v)
}

// NewEncoder implements Interface
func (StdJSON) NewEncoder(writer io.Writer) Encoder {
	return encoder.NewStreamEncoder(writer)
}

// NewDecoder implements Interface
func (StdJSON) NewDecoder(reader io.Reader) Decoder {
	return decoder.NewStreamDecoder(reader)
}

// Indent implements Interface
func (StdJSON) Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
}

// Sonic implements Interface via jsoniter
type Sonic struct {
	sonic.API
}

// Marshal implements Interface
func (s Sonic) Marshal(v interface{}) ([]byte, error) {
	return s.Marshal(v)
}

// Unmarshal implements Interface
func (s Sonic) Unmarshal(data []byte, v interface{}) error {
	return s.Unmarshal(data, v)
}

// NewEncoder implements Interface
func (s Sonic) NewEncoder(writer io.Writer) Encoder {
	return s.NewEncoder(writer)
}

// NewDecoder implements Interface
func (s Sonic) NewDecoder(reader io.Reader) Decoder {
	return s.NewDecoder(reader)
}

// Indent implements Interface, since jsoniter don't support Indent, just use encoding/json's
func (s Sonic) Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return json.Indent(dst, src, prefix, indent)
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

// UnmarshalHandleDoubleEncode - due to a bug in xorm (see https://gitea.com/xorm/xorm/pulls/1957) - it's
// possible that a Blob may be double encoded or gain an unwanted prefix of 0xff 0xfe.
func UnmarshalHandleDoubleEncode(bs []byte, v interface{}) error {
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
