// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"reflect"
	"time"
)

// Zeroable represents an interface which could know if it's a zero value
type Zeroable interface {
	IsZero() bool
}

var nilTime *time.Time

// IsZero returns false if k is nil or has a zero value
func IsZero(k interface{}) bool {
	if k == nil {
		return true
	}

	switch t := k.(type) {
	case int:
		return t == 0
	case int8:
		return t == 0
	case int16:
		return t == 0
	case int32:
		return t == 0
	case int64:
		return t == 0
	case uint:
		return t == 0
	case uint8:
		return t == 0
	case uint16:
		return t == 0
	case uint32:
		return t == 0
	case uint64:
		return t == 0
	case float32:
		return t == 0
	case float64:
		return t == 0
	case bool:
		return !t
	case string:
		return t == ""
	case *time.Time:
		return t == nilTime || IsTimeZero(*t)
	case time.Time:
		return IsTimeZero(t)
	case Zeroable:
		return k.(Zeroable) == nil || k.(Zeroable).IsZero()
	case reflect.Value: // for go version less than 1.13 because reflect.Value has no method IsZero
		return IsValueZero(k.(reflect.Value))
	}

	return IsValueZero(reflect.ValueOf(k))
}

var zeroType = reflect.TypeOf((*Zeroable)(nil)).Elem()

// IsValueZero returns true if the reflect Value is a zero
func IsValueZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice:
		return v.IsNil()
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint, reflect.Uint64:
		return v.Uint() == 0
	case reflect.String:
		return v.Len() == 0
	case reflect.Ptr:
		if v.IsNil() {
			return true
		}
		return IsValueZero(v.Elem())
	case reflect.Struct:
		return IsStructZero(v)
	case reflect.Array:
		return IsArrayZero(v)
	}
	return false
}

// IsStructZero returns true if the Value is a struct and all fields is zero
func IsStructZero(v reflect.Value) bool {
	if !v.IsValid() || v.NumField() == 0 {
		return true
	}

	if v.Type().Implements(zeroType) {
		f := v.MethodByName("IsZero")
		if f.IsValid() {
			res := f.Call(nil)
			return len(res) == 1 && res[0].Bool()
		}
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Ptr:
			field = field.Elem()
			fallthrough
		case reflect.Struct:
			if !IsStructZero(field) {
				return false
			}
		default:
			if field.CanInterface() && !IsZero(field.Interface()) {
				return false
			}
		}
	}
	return true
}

// IsArrayZero returns true is a slice of array is zero
func IsArrayZero(v reflect.Value) bool {
	if !v.IsValid() || v.Len() == 0 {
		return true
	}

	for i := 0; i < v.Len(); i++ {
		if !IsZero(v.Index(i).Interface()) {
			return false
		}
	}

	return true
}

// represents all zero times
const (
	ZeroTime0 = "0000-00-00 00:00:00"
	ZeroTime1 = "0001-01-01 00:00:00"
)

// IsTimeZero return true if a time is zero
func IsTimeZero(t time.Time) bool {
	return t.IsZero() || t.Format("2006-01-02 15:04:05") == ZeroTime0 ||
		t.Format("2006-01-02 15:04:05") == ZeroTime1
}
