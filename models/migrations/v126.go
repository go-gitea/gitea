// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/builder"
	"xorm.io/xorm"
)

func fixTopicRepositoryCount(x *xorm.Engine) error {
	type Topic struct {
		ID        int64
		RepoCount int
	}

	_, err := x.Exec(builder.Delete(builder.NotIn("`repo_id`", builder.Select("`id`").From("`repository`"))).From("`repo_topic`"))
	if err != nil {
		return err
	}

	var last int
	const batchSize = 50
	for {
		var results = make([]Topic, 0, batchSize)
		err := x.OrderBy("id").
			Limit(batchSize, last).
			Find(&results)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			break
		}
		last += len(results)

		for _, res := range results {
			_, err := x.ID(res.ID).Cols("repo_count").
				SetExpr("repo_count", builder.Select("count(*)").From("repo_topic").Where(
					builder.Eq{"topic_id": res.ID},
				)).Update(res)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
