// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/util"
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

// JoinInt64 joins a slice of int64 values into a comma-separated string.
func (su *SliceUtils) JoinInt64(values []int64) string {
	if len(values) == 0 {
		return ""
	}
	strs := make([]string, len(values))
	for i, v := range values {
		strs[i] = strconv.FormatInt(v, 10)
	}
	return strings.Join(strs, ",")
}

func (su *SliceUtils) JoinToggleIDs(values []int64, target int64) (ret struct {
	IsIncluded bool
	ToggledIDs string
},
) {
	ret.IsIncluded = slices.Contains(values, target)
	if ret.IsIncluded {
		ret.ToggledIDs = su.JoinInt64(util.SliceRemoveAll(slices.Clone(values), target))
	} else {
		ret.ToggledIDs = su.JoinInt64(append(values, target))
	}
	return ret
}
