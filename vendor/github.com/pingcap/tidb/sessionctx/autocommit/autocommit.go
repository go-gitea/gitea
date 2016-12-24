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

package autocommit

import (
	"github.com/pingcap/tidb/context"
)

// Checker is the interface checks if it should autocommit in the context.
// TODO: Choose a better name.
type Checker interface {
	// ShouldAutocommit returns true if it should autocommit in the context.
	ShouldAutocommit(ctx context.Context) bool
}

// keyType is a dummy type to avoid naming collision in context.
type keyType int

// String defines a Stringer function for debugging and pretty printing.
func (k keyType) String() string {
	return "autocommit_checker"
}

const key keyType = 0

// BindAutocommitChecker binds autocommit checker to context.
func BindAutocommitChecker(ctx context.Context, checker Checker) {
	ctx.SetValue(key, checker)
}

// ShouldAutocommit gets checker from ctx and checks if it should autocommit.
func ShouldAutocommit(ctx context.Context) bool {
	v, ok := ctx.Value(key).(Checker)
	if !ok {
		panic("Miss autocommit checker")
	}
	return v.ShouldAutocommit(ctx)
}
