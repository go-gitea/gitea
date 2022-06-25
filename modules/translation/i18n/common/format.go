// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"reflect"
)

// Format formats provided arguments for a given translated message
func Format(msg string, args ...interface{}) (string, error) {
	var err error
	if len(args) > 0 {
		fmtArgs := make([]interface{}, 0, len(args))
		for _, arg := range args {
			val := reflect.ValueOf(arg)
			if val.Kind() == reflect.Slice {
				// before, it can accept Tr(lang, key, a, [b, c], d, [e, f]) as Sprintf(msg, a, b, c, d, e, f), it's an unstable behavior
				// now, we restrict the strange behavior and only support:
				// 1. Tr(lang, key, [slice-items]) as Sprintf(msg, items...)
				// 2. Tr(lang, key, args...) as Sprintf(msg, args...)
				if len(args) == 1 {
					for i := 0; i < val.Len(); i++ {
						fmtArgs = append(fmtArgs, val.Index(i).Interface())
					}
				} else {
					err = ErrUncertainArguments
					break
				}
			} else {
				fmtArgs = append(fmtArgs, arg)
			}
		}
		return fmt.Sprintf(msg, fmtArgs...), err
	}
	return msg, err
}
