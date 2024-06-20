// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"fmt"
	"reflect"
)

type SliceUtils struct{}

func NewSliceUtils() *SliceUtils {
	return &SliceUtils{}
}

func (su *SliceUtils) Contains(s, v any) bool {
	if s == nil {
		return false
	}
	sv := reflect.ValueOf(s)
	if sv.Kind() != reflect.Slice && sv.Kind() != reflect.Array {
		panic(fmt.Sprintf("invalid type, expected slice or array, but got: %T", s))
	}
	for i := 0; i < sv.Len(); i++ {
		it := sv.Index(i)
		if !it.CanInterface() {
			continue
		}
		if it.Interface() == v {
			return true
		}
	}
	return false
}

func (su *SliceUtils) Range(args ...int) (ret []int) {
	var start, stop int
	step := 1
	if len(args) == 1 {
		stop = args[0]
	} else if len(args) == 2 {
		start, stop = args[0], args[1]
	} else if len(args) == 3 {
		start, stop, step = args[0], args[1], args[2]
	} else {
		panic(fmt.Sprintf("invalid number of Range arguments: %d", len(args)))
	}
	ret = make([]int, 0, (stop-start)/step)
	for i := start; i < stop; i += step {
		ret = append(ret, i)
	}
	return ret
}
