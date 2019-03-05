// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"reflect"

	"github.com/go-xorm/core"
)

var (
	ptrPkType = reflect.TypeOf(&core.PK{})
	pkType    = reflect.TypeOf(core.PK{})
)
