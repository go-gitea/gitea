// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"xorm.io/builder"
	"xorm.io/xorm"
)

func FixTopicRepositoryCount(x *xorm.Engine) error {
	_, err := x.Exec(builder.Delete(builder.NotIn("`repo_id`", builder.Select("`id`").From("`repository`"))).From("`repo_topic`"))
	if err != nil {
		return err
	}

	_, err = x.Exec(builder.Update(
		builder.Eq{
			"`repo_count`": builder.Select("count(*)").From("`repo_topic`").Where(builder.Eq{
				"`repo_topic`.`topic_id`": builder.Expr("`topic`.`id`"),
			}),
		}).From("`topic`").Where(builder.Eq{"'1'": "1"}))
	return err
}
