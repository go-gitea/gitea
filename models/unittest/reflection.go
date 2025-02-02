// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"fmt"
	"reflect"
)

func fieldByName(v reflect.Value, field string) reflect.Value {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	f := v.FieldByName(field)
	if !f.IsValid() {
		panic(fmt.Errorf("can not read %s for %v", field, v))
	}
	return f
}

type reflectionValue struct {
	v reflect.Value
}

func reflectionWrap(v any) *reflectionValue {
	return &reflectionValue{v: reflect.ValueOf(v)}
}

func (rv *reflectionValue) int(field string) int {
	return int(fieldByName(rv.v, field).Int())
}

func (rv *reflectionValue) str(field string) string {
	return fieldByName(rv.v, field).String()
}

func (rv *reflectionValue) bool(field string) bool {
	return fieldByName(rv.v, field).Bool()
}
