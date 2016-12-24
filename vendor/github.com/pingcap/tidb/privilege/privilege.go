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

package privilege

import (
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
)

type keyType int

func (k keyType) String() string {
	return "privilege-key"
}

// Checker is the interface for check privileges.
type Checker interface {
	// Check checks privilege.
	// If tbl is nil, only check global/db scope privileges.
	// If tbl is not nil, check global/db/table scope privileges.
	Check(ctx context.Context, db *model.DBInfo, tbl *model.TableInfo, privilege mysql.PrivilegeType) (bool, error)
	// Show granted privileges for user.
	ShowGrants(ctx context.Context, user string) ([]string, error)
}

const key keyType = 0

// BindPrivilegeChecker binds Checker to context.
func BindPrivilegeChecker(ctx context.Context, pc Checker) {
	ctx.SetValue(key, pc)
}

// GetPrivilegeChecker gets Checker from context.
func GetPrivilegeChecker(ctx context.Context) Checker {
	if v, ok := ctx.Value(key).(Checker); ok {
		return v
	}
	return nil
}
