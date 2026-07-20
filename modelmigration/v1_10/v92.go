// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/builder"
)

func RemoveLingeringIndexStatus(x base.EngineMigration) error {
	_, err := x.Exec(builder.Delete(builder.NotIn("`repo_id`", builder.Select("`id`").From("`repository`"))).From("`repo_indexer_status`"))
	return err
}
