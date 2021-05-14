// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"reflect"
)

// ReflectValue returns value of a bean
func ReflectValue(bean interface{}) reflect.Value {
	return reflect.Indirect(reflect.ValueOf(bean))
}
