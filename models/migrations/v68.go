// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	"github.com/go-xorm/xorm"
)

func reformatAndRemoveIncorrectTopics(x *xorm.Engine) (err error) {
	log.Info("This migration could take up to minutes, please be patient.")
	type Topic struct {
		ID   int64
		Name string `xorm:"unique"`
	}

	sess := x.NewSession()
	defer sess.Close()

	const batchSize = 100
	touchedRepo := make(map[int64]struct{})
	topics := make([]*Topic, 0, batchSize)
	delTopicIDs := make([]int64, 0, batchSize)
	ids := make([]int64, 0, 30)

	if err := sess.Begin(); err != nil {
		return err
	}
	log.Info("Validating existed topics...")
	for start := 0; ; start += batchSize {
		topics = topics[:0]
		if err := sess.Asc("id").Limit(batchSize, start).Find(&topics); err != nil {
			return err
		}
		if len(topics) == 0 {
			break
		}
		for _, topic := range topics {
			if models.ValidateTopic(topic.Name) {
				continue
			}
			topic.Name = strings.Replace(strings.TrimSpace(strings.ToLower(topic.Name)), " ", "-", -1)

			if err := sess.Table("repo_topic").Cols("repo_id").
				Where("topic_id = ?", topic.ID).Find(&ids); err != nil {
				return err
			}
			for _, id := range ids {
				touchedRepo[id] = struct{}{}
			}

			if models.ValidateTopic(topic.Name) {
				log.Info("Updating topic: id = %v, name = %v", topic.ID, topic.Name)
				if _, err := sess.Table("topic").ID(topic.ID).
					Update(&Topic{Name: topic.Name}); err != nil {
					return err
				}
			} else {
				delTopicIDs = append(delTopicIDs, topic.ID)
			}
		}
	}

	log.Info("Deleting incorrect topics...")
	for start := 0; ; start += batchSize {
		if (start + batchSize) < len(delTopicIDs) {
			ids = delTopicIDs[start:(start + batchSize)]
		} else {
			ids = delTopicIDs[start:]
		}

		log.Info("Deleting 'repo_topic' rows for topics with ids = %v", ids)
		if _, err := sess.In("topic_id", ids).Delete(&models.RepoTopic{}); err != nil {
			return err
		}

		log.Info("Deleting topics with id = %v", ids)
		if _, err := sess.In("id", ids).Delete(&Topic{}); err != nil {
			return err
		}

		if len(ids) < batchSize {
			break
		}
	}

	repoTopics := make([]*models.RepoTopic, 0, batchSize)
	delRepoTopics := make([]*models.RepoTopic, 0, batchSize)
	tmpRepoTopics := make([]*models.RepoTopic, 0, 30)

	log.Info("Checking the number of topics in the repositories...")
	for start := 0; ; start += batchSize {
		repoTopics = repoTopics[:0]
		if err := sess.Cols("repo_id").Asc("repo_id").Limit(batchSize, start).
			GroupBy("repo_id").Having("COUNT(*) > 25").Find(&repoTopics); err != nil {
			return err
		}
		if len(repoTopics) == 0 {
			break
		}

		log.Info("Number of repositories with more than 25 topics: %v", len(repoTopics))
		for _, repoTopic := range repoTopics {
			touchedRepo[repoTopic.RepoID] = struct{}{}

			tmpRepoTopics = tmpRepoTopics[:0]
			if err := sess.Where("repo_id = ?", repoTopic.RepoID).Find(&tmpRepoTopics); err != nil {
				return err
			}

			log.Info("Repository with id = %v has %v topics", repoTopic.RepoID, len(tmpRepoTopics))

			for i := len(tmpRepoTopics) - 1; i > 24; i-- {
				delRepoTopics = append(delRepoTopics, tmpRepoTopics[i])
			}
		}
	}

	log.Info("Deleting superfluous topics for repositories (more than 25 topics)...")
	for _, repoTopic := range delRepoTopics {
		log.Info("Deleting 'repo_topic' rows for 'repository' with id = %v. Topic id = %v",
			repoTopic.RepoID, repoTopic.TopicID)

		if _, err := sess.Where("repo_id = ? AND topic_id = ?", repoTopic.RepoID,
			repoTopic.TopicID).Delete(&models.RepoTopic{}); err != nil {
			return err
		}
		if _, err := sess.Exec(
			"UPDATE topic SET repo_count = (SELECT repo_count FROM topic WHERE id = ?) - 1 WHERE id = ?",
			repoTopic.TopicID, repoTopic.TopicID); err != nil {
			return err
		}
	}

	topicNames := make([]string, 0, 30)
	log.Info("Updating repositories 'topics' fields...")
	for repoID := range touchedRepo {
		if err := sess.Table("topic").Cols("name").
			Join("INNER", "repo_topic", "topic.id = repo_topic.topic_id").
			Where("repo_topic.repo_id = ?", repoID).Find(&topicNames); err != nil {
			return err
		}
		log.Info("Updating 'topics' field for repository with id = %v", repoID)
		if _, err := sess.ID(repoID).Cols("topics").
			Update(&models.Repository{Topics: topicNames}); err != nil {
			return err
		}
	}
	if err := sess.Commit(); err != nil {
		return err
	}

	return nil
}
