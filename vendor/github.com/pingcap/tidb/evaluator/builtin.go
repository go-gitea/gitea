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
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/util/types"
)

// OldFunc is for a old builtin function.
type OldFunc struct {
	// F is the specific calling function.
	F func([]interface{}, context.Context) (interface{}, error)
	// MinArgs is the minimal arguments needed,
	MinArgs int
	// MaxArgs is the maximal arguments needed, -1 for infinity.
	MaxArgs int
	// IsStatic shows whether this function can be called statically.
	IsStatic bool
	// IsAggregate represents whether this function is an aggregate function or not.
	IsAggregate bool
}

// Func is for a builtin function.
type Func struct {
	// F is the specific calling function.
	F func([]types.Datum, context.Context) (types.Datum, error)
	// MinArgs is the minimal arguments needed,
	MinArgs int
	// MaxArgs is the maximal arguments needed, -1 for infinity.
	MaxArgs int
}

// OldFuncs holds all has old registered builtin functions.
var OldFuncs = map[string]OldFunc{
	// control functions
	"if":     {builtinIf, 3, 3, true, false},
	"ifnull": {builtinIfNull, 2, 2, true, false},
	"nullif": {builtinNullIf, 2, 2, true, false},

	// string functions
	"replace":         {builtinReplace, 3, 3, true, false},
	"strcmp":          {builtinStrcmp, 2, 2, true, false},
	"convert":         {builtinConvert, 2, 2, true, false},
	"substring":       {builtinSubstring, 2, 3, true, false},
	"substring_index": {builtinSubstringIndex, 3, 3, true, false},
	"locate":          {builtinLocate, 2, 3, true, false},
	"trim":            {builtinTrim, 1, 3, true, false},

	// information functions
	"current_user":  {builtinCurrentUser, 0, 0, false, false},
	"database":      {builtinDatabase, 0, 0, false, false},
	"found_rows":    {builtinFoundRows, 0, 0, false, false},
	"user":          {builtinUser, 0, 0, false, false},
	"connection_id": {builtinConnectionID, 0, 0, true, false},
	"version":       {builtinVersion, 0, 0, true, false},
}

// Funcs holds all registered builtin functions.
var Funcs = map[string]Func{
	// common functions
	"coalesce": {builtinCoalesce, 1, -1},

	// math functions
	"abs":   {builtinAbs, 1, 1},
	"pow":   {builtinPow, 2, 2},
	"power": {builtinPow, 2, 2},
	"rand":  {builtinRand, 0, 1},

	// time functions
	"curdate":           {builtinCurrentDate, 0, 0},
	"current_date":      {builtinCurrentDate, 0, 0},
	"current_time":      {builtinCurrentTime, 0, 1},
	"current_timestamp": {builtinNow, 0, 1},
	"curtime":           {builtinCurrentTime, 0, 1},
	"date":              {builtinDate, 1, 1},
	"day":               {builtinDay, 1, 1},
	"dayname":           {builtinDayName, 1, 1},
	"dayofmonth":        {builtinDayOfMonth, 1, 1},
	"dayofweek":         {builtinDayOfWeek, 1, 1},
	"dayofyear":         {builtinDayOfYear, 1, 1},
	"hour":              {builtinHour, 1, 1},
	"microsecond":       {builtinMicroSecond, 1, 1},
	"minute":            {builtinMinute, 1, 1},
	"month":             {builtinMonth, 1, 1},
	"now":               {builtinNow, 0, 1},
	"second":            {builtinSecond, 1, 1},
	"sysdate":           {builtinSysDate, 0, 1},
	"week":              {builtinWeek, 1, 2},
	"weekday":           {builtinWeekDay, 1, 1},
	"weekofyear":        {builtinWeekOfYear, 1, 1},
	"year":              {builtinYear, 1, 1},
	"yearweek":          {builtinYearWeek, 1, 2},
	"extract":           {builtinExtract, 2, 2},
	"date_arith":        {builtinDateArith, 3, 3},

	// string functions
	"concat":    {builtinConcat, 1, -1},
	"concat_ws": {builtinConcatWS, 2, -1},
	"left":      {builtinLeft, 2, 2},
	"length":    {builtinLength, 1, 1},
	"lower":     {builtinLower, 1, 1},
	"repeat":    {builtinRepeat, 2, 2},
	"upper":     {builtinUpper, 1, 1},
}

// See: http://dev.mysql.com/doc/refman/5.7/en/comparison-operators.html#function_coalesce
func builtinCoalesce(args []types.Datum, ctx context.Context) (d types.Datum, err error) {
	for _, d = range args {
		if d.Kind() != types.KindNull {
			return d, nil
		}
	}
	return d, nil
}
