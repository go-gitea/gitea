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

func IsZero(k interface{}) bool {
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
	case Zeroable:
		return k.(Zeroable).IsZero()
	}
	return false
}

func IsValueZero(v reflect.Value) bool {
	if IsZero(v.Interface()) {
		return true
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func IsStructZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
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
