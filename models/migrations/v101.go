// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "github.com/go-xorm/xorm"

func addWhitelistDeployKeysToBranches(x *xorm.Engine) error {
	type ProtectedBranch struct {
		ID                  int64
		WhitelistDeployKeys bool
	}

	return x.Sync2(new(ProtectedBranch))
}
