// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"reflect"
	"time"
)

type Zeroable interface {
	IsZero() bool
}

var nilTime *time.Time

// IsZero returns false if k is nil or has a zero value
func IsZero(k interface{}) bool {
	if k == nil {
		return true
	}

	switch k.(type) {
	case int:
		return k.(int) == 0
	case int8:
		return k.(int8) == 0
	case int16:
		return k.(int16) == 0
	case int32:
		return k.(int32) == 0
	case int64:
		return k.(int64) == 0
	case uint:
		return k.(uint) == 0
	case uint8:
		return k.(uint8) == 0
	case uint16:
		return k.(uint16) == 0
	case uint32:
		return k.(uint32) == 0
	case uint64:
		return k.(uint64) == 0
	case float32:
		return k.(float32) == 0
	case float64:
		return k.(float64) == 0
	case bool:
		return k.(bool) == false
	case string:
		return k.(string) == ""
	case *time.Time:
		return k.(*time.Time) == nilTime || IsTimeZero(*k.(*time.Time))
	case time.Time:
		return IsTimeZero(k.(time.Time))
	case Zeroable:
		return k.(Zeroable) == nil || k.(Zeroable).IsZero()
	case reflect.Value: // for go version less than 1.13 because reflect.Value has no method IsZero
		return IsValueZero(k.(reflect.Value))
	}

	return IsValueZero(reflect.ValueOf(k))
}

var zeroType = reflect.TypeOf((*Zeroable)(nil)).Elem()

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

const (
	ZeroTime0 = "0000-00-00 00:00:00"
	ZeroTime1 = "0001-01-01 00:00:00"
)

func IsTimeZero(t time.Time) bool {
	return t.IsZero() || t.Format("2006-01-02 15:04:05") == ZeroTime0 ||
		t.Format("2006-01-02 15:04:05") == ZeroTime1
}
