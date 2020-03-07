// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"reflect"

	"xorm.io/xorm/schemas"
)

var (
	ptrPkType = reflect.TypeOf(&schemas.PK{})
	pkType    = reflect.TypeOf(schemas.PK{})
)
