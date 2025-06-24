// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"xorm.io/builder"
	"xorm.io/xorm"
)

func RemoveLingeringIndexStatus(x *xorm.Engine) error {
	_, err := x.Exec(builder.Delete(builder.NotIn("`repo_id`", builder.Select("`id`").From("`repository`"))).From("`repo_indexer_status`"))
	return err
}
