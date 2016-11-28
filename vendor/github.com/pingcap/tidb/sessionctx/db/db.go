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

package db

import (
	"github.com/pingcap/tidb/context"
)

// currentDBKeyType is a dummy type to avoid naming collision in context.
type currentDBKeyType int

// String defines a Stringer function for debugging and pretty printing.
func (k currentDBKeyType) String() string {
	return "current_db"
}

const currentDBKey currentDBKeyType = 0

// BindCurrentSchema saves parameter schema as current schema name value into context.
func BindCurrentSchema(ctx context.Context, schema string) {
	ctx.SetValue(currentDBKey, schema)
}

// GetCurrentSchema gets current schema name from context.
func GetCurrentSchema(ctx context.Context) string {
	v, ok := ctx.Value(currentDBKey).(string)
	if !ok {
		return ""
	}
	return v
}
