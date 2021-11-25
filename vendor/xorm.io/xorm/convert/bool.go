// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package convert

import (
	"database/sql"
	"fmt"
	"strconv"
)

// AsBool convert interface as bool
func AsBool(src interface{}) (bool, error) {
	switch v := src.(type) {
	case bool:
		return v, nil
	case *bool:
		return *v, nil
	case *sql.NullBool:
		return v.Bool, nil
	case int64:
		return v > 0, nil
	case int:
		return v > 0, nil
	case int8:
		return v > 0, nil
	case int16:
		return v > 0, nil
	case int32:
		return v > 0, nil
	case []byte:
		if len(v) == 0 {
			return false, nil
		}
		if v[0] == 0x00 {
			return false, nil
		} else if v[0] == 0x01 {
			return true, nil
		}
		return strconv.ParseBool(string(v))
	case string:
		return strconv.ParseBool(v)
	case *sql.NullInt64:
		return v.Int64 > 0, nil
	case *sql.NullInt32:
		return v.Int32 > 0, nil
	default:
		return false, fmt.Errorf("unknow type %T as bool", src)
	}
}
