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

// Package forupdate record information for "select ... for update" statement
package forupdate

import "github.com/pingcap/tidb/context"

// A dummy type to avoid naming collision in context.
type forupdateKeyType int

// String defines a Stringer function for debugging and pretty printing.
func (k forupdateKeyType) String() string {
	return "for update"
}

// ForUpdateKey is used to retrive "select for update" statement information
const ForUpdateKey forupdateKeyType = 0

// SetForUpdate set "select for update" flag.
func SetForUpdate(ctx context.Context) {
	ctx.SetValue(ForUpdateKey, true)
}
