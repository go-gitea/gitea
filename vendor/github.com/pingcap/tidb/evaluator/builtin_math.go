// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package evaluator

import (
	"math"
	"math/rand"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/util/types"
)

// see https://dev.mysql.com/doc/refman/5.7/en/mathematical-functions.html

func builtinAbs(args []types.Datum, _ context.Context) (d types.Datum, err error) {
	d = args[0]
	switch d.Kind() {
	case types.KindNull:
		return d, nil
	case types.KindUint64:
		return d, nil
	case types.KindInt64:
		iv := d.GetInt64()
		if iv >= 0 {
			d.SetInt64(iv)
			return d, nil
		}
		d.SetInt64(-iv)
		return d, nil
	default:
		// we will try to convert other types to float
		// TODO: if time has no precision, it will be a integer
		f, err := d.ToFloat64()
		d.SetFloat64(math.Abs(f))
		return d, errors.Trace(err)
	}
}

func builtinRand(args []types.Datum, _ context.Context) (d types.Datum, err error) {
	if len(args) == 1 && args[0].Kind() != types.KindNull {
		seed, err := args[0].ToInt64()
		if err != nil {
			d.SetNull()
			return d, errors.Trace(err)
		}

		rand.Seed(seed)
	}
	d.SetFloat64(rand.Float64())
	return d, nil
}

func builtinPow(args []types.Datum, _ context.Context) (d types.Datum, err error) {
	x, err := args[0].ToFloat64()
	if err != nil {
		d.SetNull()
		return d, errors.Trace(err)
	}

	y, err := args[1].ToFloat64()
	if err != nil {
		d.SetNull()
		return d, errors.Trace(err)
	}
	d.SetFloat64(math.Pow(x, y))
	return d, nil
}
