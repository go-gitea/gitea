// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Bool return address of bool value
func Bool(v bool) *bool {
	return &v
}

// String return address of string value
func String(v string) *string {
	return &v
}

// Int64 return address of int64 value
func Int64(v int64) *int64 {
	return &v
}
