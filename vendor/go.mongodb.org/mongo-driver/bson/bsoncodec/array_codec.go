// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncodec

import (
	"reflect"

	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

// ArrayCodec is the Codec used for bsoncore.Array values.
type ArrayCodec struct{}

var defaultArrayCodec = NewArrayCodec()

// NewArrayCodec returns an ArrayCodec.
func NewArrayCodec() *ArrayCodec {
	return &ArrayCodec{}
}

// EncodeValue is the ValueEncoder for bsoncore.Array values.
func (ac *ArrayCodec) EncodeValue(ec EncodeContext, vw bsonrw.ValueWriter, val reflect.Value) error {
	if !val.IsValid() || val.Type() != tCoreArray {
		return ValueEncoderError{Name: "CoreArrayEncodeValue", Types: []reflect.Type{tCoreArray}, Received: val}
	}

	arr := val.Interface().(bsoncore.Array)
	return bsonrw.Copier{}.CopyArrayFromBytes(vw, arr)
}

// DecodeValue is the ValueDecoder for bsoncore.Array values.
func (ac *ArrayCodec) DecodeValue(dc DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	if !val.CanSet() || val.Type() != tCoreArray {
		return ValueDecoderError{Name: "CoreArrayDecodeValue", Types: []reflect.Type{tCoreArray}, Received: val}
	}

	if val.IsNil() {
		val.Set(reflect.MakeSlice(val.Type(), 0, 0))
	}

	val.SetLen(0)
	arr, err := bsonrw.Copier{}.AppendArrayBytes(val.Interface().(bsoncore.Array), vr)
	val.Set(reflect.ValueOf(arr))
	return err
}
