// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"xorm.io/xorm"
)

func FixRepoTopics(x *xorm.Engine) error {
	type Topic struct { //nolint:unused
		ID        int64  `xorm:"pk autoincr"`
		Name      string `xorm:"UNIQUE VARCHAR(25)"`
		RepoCount int
	}

	type RepoTopic struct { //nolint:unused
		RepoID  int64 `xorm:"pk"`
		TopicID int64 `xorm:"pk"`
	}

	type Repository struct {
		ID     int64    `xorm:"pk autoincr"`
		Topics []string `xorm:"TEXT JSON"`
	}

	const batchSize = 100
	sess := x.NewSession()
	defer sess.Close()
	repos := make([]*Repository, 0, batchSize)
	topics := make([]string, 0, batchSize)
	for start := 0; ; start += batchSize {
		repos = repos[:0]

		if err := sess.Begin(); err != nil {
			return err
		}

		if err := sess.Limit(batchSize, start).Find(&repos); err != nil {
			return err
		}

		if len(repos) == 0 {
			break
		}

		for _, repo := range repos {
			topics = topics[:0]
			if err := sess.Select("name").Table("topic").
				Join("INNER", "repo_topic", "repo_topic.topic_id = topic.id").
				Where("repo_topic.repo_id = ?", repo.ID).Desc("topic.repo_count").Find(&topics); err != nil {
				return err
			}
			repo.Topics = topics
			if _, err := sess.ID(repo.ID).Cols("topics").Update(repo); err != nil {
				return err
			}
		}

		if err := sess.Commit(); err != nil {
			return err
		}
	}

	return nil
}
