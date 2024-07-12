// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rubygems

import (
	"bufio"
	"bytes"
	"io"
	"reflect"

	"code.gitea.io/gitea/modules/util"
)

const (
	majorVersion = 4
	minorVersion = 8

	typeNil         = '0'
	typeTrue        = 'T'
	typeFalse       = 'F'
	typeFixnum      = 'i'
	typeString      = '"'
	typeSymbol      = ':'
	typeSymbolLink  = ';'
	typeArray       = '['
	typeIVar        = 'I'
	typeUserMarshal = 'U'
	typeUserDef     = 'u'
	typeObject      = 'o'
)

var (
	// ErrUnsupportedType indicates an unsupported type
	ErrUnsupportedType = util.NewInvalidArgumentErrorf("type is unsupported")
	// ErrInvalidIntRange indicates an invalid number range
	ErrInvalidIntRange = util.NewInvalidArgumentErrorf("number is not in valid range")
)

// RubyUserMarshal is a Ruby object that has a marshal_load function.
type RubyUserMarshal struct {
	Name  string
	Value any
}

// RubyUserDef is a Ruby object that has a _load function.
type RubyUserDef struct {
	Name  string
	Value any
}

// RubyObject is a default Ruby object.
type RubyObject struct {
	Name   string
	Member map[string]any
}

// MarshalEncoder mimics Rubys Marshal class.
// Note: Only supports types used by the RubyGems package registry.
type MarshalEncoder struct {
	w       *bufio.Writer
	symbols map[string]int
}

// NewMarshalEncoder creates a new MarshalEncoder
func NewMarshalEncoder(w io.Writer) *MarshalEncoder {
	return &MarshalEncoder{
		w:       bufio.NewWriter(w),
		symbols: map[string]int{},
	}
}

// Encode encodes the given type
func (e *MarshalEncoder) Encode(v any) error {
	if _, err := e.w.Write([]byte{majorVersion, minorVersion}); err != nil {
		return err
	}

	if err := e.marshal(v); err != nil {
		return err
	}

	return e.w.Flush()
}

func (e *MarshalEncoder) marshal(v any) error {
	if v == nil {
		return e.marshalNil()
	}

	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	if typ.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	switch typ.Kind() {
	case reflect.Bool:
		return e.marshalBool(val.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return e.marshalInt(val.Int())
	case reflect.String:
		return e.marshalString(val.String())
	case reflect.Slice, reflect.Array:
		return e.marshalArray(val)
	}

	switch typ.Name() {
	case "RubyUserMarshal":
		return e.marshalUserMarshal(val.Interface().(RubyUserMarshal))
	case "RubyUserDef":
		return e.marshalUserDef(val.Interface().(RubyUserDef))
	case "RubyObject":
		return e.marshalObject(val.Interface().(RubyObject))
	}

	return ErrUnsupportedType
}

func (e *MarshalEncoder) marshalNil() error {
	return e.w.WriteByte(typeNil)
}

func (e *MarshalEncoder) marshalBool(b bool) error {
	if b {
		return e.w.WriteByte(typeTrue)
	}
	return e.w.WriteByte(typeFalse)
}

func (e *MarshalEncoder) marshalInt(i int64) error {
	if err := e.w.WriteByte(typeFixnum); err != nil {
		return err
	}

	return e.marshalIntInternal(i)
}

func (e *MarshalEncoder) marshalIntInternal(i int64) error {
	if i == 0 {
		return e.w.WriteByte(0)
	} else if 0 < i && i < 123 {
		return e.w.WriteByte(byte(i + 5))
	} else if -124 < i && i <= -1 {
		return e.w.WriteByte(byte(i - 5))
	}

	var length int
	if 122 < i && i <= 0xff {
		length = 1
	} else if 0xff < i && i <= 0xffff {
		length = 2
	} else if 0xffff < i && i <= 0xffffff {
		length = 3
	} else if 0xffffff < i && i <= 0x3fffffff {
		length = 4
	} else if -0x100 <= i && i < -123 {
		length = -1
	} else if -0x10000 <= i && i < -0x100 {
		length = -2
	} else if -0x1000000 <= i && i < -0x100000 {
		length = -3
	} else if -0x40000000 <= i && i < -0x1000000 {
		length = -4
	} else {
		return ErrInvalidIntRange
	}

	if err := e.w.WriteByte(byte(length)); err != nil {
		return err
	}
	if length < 0 {
		length = -length
	}

	for c := 0; c < length; c++ {
		if err := e.w.WriteByte(byte(i >> uint(8*c) & 0xff)); err != nil {
			return err
		}
	}

	return nil
}

func (e *MarshalEncoder) marshalString(str string) error {
	if err := e.w.WriteByte(typeIVar); err != nil {
		return err
	}

	if err := e.marshalRawString(str); err != nil {
		return err
	}

	if err := e.marshalIntInternal(1); err != nil {
		return err
	}

	if err := e.marshalSymbol("E"); err != nil {
		return err
	}

	return e.marshalBool(true)
}

func (e *MarshalEncoder) marshalRawString(str string) error {
	if err := e.w.WriteByte(typeString); err != nil {
		return err
	}

	if err := e.marshalIntInternal(int64(len(str))); err != nil {
		return err
	}

	_, err := e.w.WriteString(str)
	return err
}

func (e *MarshalEncoder) marshalSymbol(str string) error {
	if index, ok := e.symbols[str]; ok {
		if err := e.w.WriteByte(typeSymbolLink); err != nil {
			return err
		}
		return e.marshalIntInternal(int64(index))
	}

	e.symbols[str] = len(e.symbols)

	if err := e.w.WriteByte(typeSymbol); err != nil {
		return err
	}

	if err := e.marshalIntInternal(int64(len(str))); err != nil {
		return err
	}

	_, err := e.w.WriteString(str)
	return err
}

func (e *MarshalEncoder) marshalArray(arr reflect.Value) error {
	if err := e.w.WriteByte(typeArray); err != nil {
		return err
	}

	length := arr.Len()

	if err := e.marshalIntInternal(int64(length)); err != nil {
		return err
	}

	for i := 0; i < length; i++ {
		if err := e.marshal(arr.Index(i).Interface()); err != nil {
			return err
		}
	}
	return nil
}

func (e *MarshalEncoder) marshalUserMarshal(userMarshal RubyUserMarshal) error {
	if err := e.w.WriteByte(typeUserMarshal); err != nil {
		return err
	}

	if err := e.marshalSymbol(userMarshal.Name); err != nil {
		return err
	}

	return e.marshal(userMarshal.Value)
}

func (e *MarshalEncoder) marshalUserDef(userDef RubyUserDef) error {
	var buf bytes.Buffer
	if err := NewMarshalEncoder(&buf).Encode(userDef.Value); err != nil {
		return err
	}

	if err := e.w.WriteByte(typeUserDef); err != nil {
		return err
	}
	if err := e.marshalSymbol(userDef.Name); err != nil {
		return err
	}
	if err := e.marshalIntInternal(int64(buf.Len())); err != nil {
		return err
	}
	_, err := e.w.Write(buf.Bytes())
	return err
}

func (e *MarshalEncoder) marshalObject(obj RubyObject) error {
	if err := e.w.WriteByte(typeObject); err != nil {
		return err
	}
	if err := e.marshalSymbol(obj.Name); err != nil {
		return err
	}
	if err := e.marshalIntInternal(int64(len(obj.Member))); err != nil {
		return err
	}
	for k, v := range obj.Member {
		if err := e.marshalSymbol(k); err != nil {
			return err
		}
		if err := e.marshal(v); err != nil {
			return err
		}
	}
	return nil
}
