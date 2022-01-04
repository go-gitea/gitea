// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

// OptionalBool convert a bool to a bool reference
func OptionalBool(v bool) *bool {
	return &v
}

// OptionalString convert a string to a string reference
func OptionalString(v string) *string {
	return &v
}

// OptionalInt64 convert a int64 to a int64 reference
func OptionalInt64(v int64) *int64 {
	return &v
}
