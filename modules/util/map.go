// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"reflect"
)

// KeysString returns a slice of keys from a map, dict must be a map
func KeysString(dict interface{}) interface{} {
	value := reflect.ValueOf(dict)
	valueType := value.Type()
	if value.Kind() == reflect.Map {
		keys := value.MapKeys()
		length := len(keys)
		resultSlice := reflect.MakeSlice(reflect.SliceOf(valueType.Key()), length, length)
		for i, key := range keys {
			resultSlice.Index(i).Set(key)
		}
		return resultSlice.Interface()
	}
	panic(fmt.Sprintf("Type %s is not supported by KeysString", valueType.String()))
}
