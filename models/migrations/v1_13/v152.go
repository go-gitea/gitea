// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint:revive // underscore in migration packages isn't a large issue

import "xorm.io/xorm"

func AddTrustModelToRepository(x *xorm.Engine) error {
	type Repository struct {
		TrustModel int
	}
	return x.Sync(new(Repository))
}
