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
	var ids []int64
	touchedRepo := make(map[int64]struct{})
	topics := make([]*Topic, 0, batchSize)

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
			if models.TopicValidator(topic.Name) {
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

			if models.TopicValidator(topic.Name) {
				log.Info("Updating topic: id = %v, name = %v", topic.ID, topic.Name)
				if _, err := sess.Table("topic").ID(topic.ID).
					Update(&Topic{Name: topic.Name}); err != nil {
					return err
				}
			} else {
				log.Info("Deleting 'repo_topic' rows for 'topic' with id = %v and topicName = %v",
					topic.ID, topic.Name)

				if _, err := sess.Where("topic_id = ?", topic.ID).
					Delete(&models.RepoTopic{}); err != nil {
					return err
				}

				log.Info("Deleting 'topic' with id = %v and topicName = %v", topic.ID, topic.Name)
				if _, err := sess.ID(topic.ID).Delete(&Topic{}); err != nil {
					return err
				}
			}
		}
	}

	repoTopics := make([]*models.RepoTopic, 0, batchSize)
	tmpRepoTopics := make([]*models.RepoTopic, 0, 25)
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
				log.Info("Deleting 'repo_topic' rows for 'repository' with id = %v. Topic id = %v",
					tmpRepoTopics[i].RepoID, tmpRepoTopics[i].TopicID)

				if _, err := sess.Where("repo_id = ? AND topic_id = ?", tmpRepoTopics[i].RepoID,
					tmpRepoTopics[i].TopicID).Delete(&models.RepoTopic{}); err != nil {
					return err
				}
				if _, err := sess.Exec(
					"UPDATE topic SET repo_count = (SELECT repo_count FROM topic WHERE id = ?) - 1 WHERE id = ?",
					tmpRepoTopics[i].TopicID, tmpRepoTopics[i].TopicID); err != nil {
					return err
				}
			}
		}
	}

	var topicNames []string
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
