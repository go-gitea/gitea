// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"xorm.io/xorm"
)

// AddHTTPSDeployKeyTable introduces the per-repository HTTPS deploy-key
// credential table. The shape here must stay in lock-step with the
// HTTPSDeployKey struct in models/asymkey.
func AddHTTPSDeployKeyTable(x *xorm.Engine) error {
	type HTTPSDeployKey struct {
		ID             int64  `xorm:"pk autoincr"`
		RepoID         int64  `xorm:"INDEX UNIQUE(s) NOT NULL"`
		Name           string `xorm:"UNIQUE(s) NOT NULL"`
		TokenHash      string `xorm:"UNIQUE NOT NULL"`
		TokenSalt      string `xorm:"NOT NULL"`
		TokenLastEight string `xorm:"INDEX"`
		Mode           int    `xorm:"NOT NULL DEFAULT 1"`
		CreatedUnix    int64  `xorm:"INDEX created"`
		UpdatedUnix    int64  `xorm:"INDEX updated"`
	}
	return x.Sync(new(HTTPSDeployKey))
}
