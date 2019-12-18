// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

var topicPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func validateTopic(topic string) bool {
	return len(topic) <= 35 && topicPattern.MatchString(topic)
}

func reformatAndRemoveIncorrectTopics(x *xorm.Engine) (err error) {
	log.Info("This migration could take up to minutes, please be patient.")

	type Topic struct {
		ID          int64
		Name        string `xorm:"UNIQUE VARCHAR(25)"`
		RepoCount   int
		CreatedUnix int64 `xorm:"INDEX created"`
		UpdatedUnix int64 `xorm:"INDEX updated"`
	}

	type RepoTopic struct {
		RepoID  int64 `xorm:"UNIQUE(s)"`
		TopicID int64 `xorm:"UNIQUE(s)"`
	}

	type Repository struct {
		ID     int64    `xorm:"pk autoincr"`
		Topics []string `xorm:"TEXT JSON"`
	}

	if err := x.Sync2(new(Topic)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	if err := x.Sync2(new(RepoTopic)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()

	const batchSize = 100
	touchedRepo := make(map[int64]struct{})
	delTopicIDs := make([]int64, 0, batchSize)

	log.Info("Validating existed topics...")
	if err := sess.Begin(); err != nil {
		return err
	}
	for start := 0; ; start += batchSize {
		topics := make([]*Topic, 0, batchSize)
		if err := x.Cols("id", "name").Asc("id").Limit(batchSize, start).Find(&topics); err != nil {
			return err
		}
		if len(topics) == 0 {
			break
		}
		for _, topic := range topics {
			if validateTopic(topic.Name) {
				continue
			}
			log.Info("Incorrect topic: id = %v, name = %q", topic.ID, topic.Name)

			topic.Name = strings.Replace(strings.TrimSpace(strings.ToLower(topic.Name)), " ", "-", -1)

			ids := make([]int64, 0, 30)
			if err := sess.Table("repo_topic").Cols("repo_id").
				Where("topic_id = ?", topic.ID).Find(&ids); err != nil {
				return err
			}
			log.Info("Touched repo ids: %v", ids)
			for _, id := range ids {
				touchedRepo[id] = struct{}{}
			}

			if validateTopic(topic.Name) {
				unifiedTopic := Topic{Name: topic.Name}
				exists, err := sess.Cols("id", "name").Get(&unifiedTopic)
				log.Info("Exists topic with the name %q? %v, id = %v", topic.Name, exists, unifiedTopic.ID)
				if err != nil {
					return err
				}
				if exists {
					log.Info("Updating repo_topic rows with topic_id = %v to topic_id = %v", topic.ID, unifiedTopic.ID)
					if _, err := sess.Where("topic_id = ? AND repo_id NOT IN "+
						"(SELECT rt1.repo_id FROM repo_topic rt1 INNER JOIN repo_topic rt2 "+
						"ON rt1.repo_id = rt2.repo_id WHERE rt1.topic_id = ? AND rt2.topic_id = ?)",
						topic.ID, topic.ID, unifiedTopic.ID).Update(&RepoTopic{TopicID: unifiedTopic.ID}); err != nil {
						return err
					}
					log.Info("Updating topic `repo_count` field")
					if _, err := sess.Exec(
						"UPDATE topic SET repo_count = (SELECT COUNT(*) FROM repo_topic WHERE topic_id = ? GROUP BY topic_id) WHERE id = ?",
						unifiedTopic.ID, unifiedTopic.ID); err != nil {
						return err
					}
				} else {
					log.Info("Updating topic: id = %v, name = %q", topic.ID, topic.Name)
					if _, err := sess.Table("topic").ID(topic.ID).
						Update(&Topic{Name: topic.Name}); err != nil {
						return err
					}
					continue
				}
			}
			delTopicIDs = append(delTopicIDs, topic.ID)
		}
	}
	if err := sess.Commit(); err != nil {
		return err
	}

	sess.Init()

	log.Info("Deleting incorrect topics...")
	if err := sess.Begin(); err != nil {
		return err
	}
	log.Info("Deleting 'repo_topic' rows for topics with ids = %v", delTopicIDs)
	if _, err := sess.In("topic_id", delTopicIDs).Delete(&RepoTopic{}); err != nil {
		return err
	}
	log.Info("Deleting topics with id = %v", delTopicIDs)
	if _, err := sess.In("id", delTopicIDs).Delete(&Topic{}); err != nil {
		return err
	}
	if err := sess.Commit(); err != nil {
		return err
	}

	delRepoTopics := make([]*RepoTopic, 0, batchSize)

	log.Info("Checking the number of topics in the repositories...")
	for start := 0; ; start += batchSize {
		repoTopics := make([]*RepoTopic, 0, batchSize)
		if err := x.Cols("repo_id").Asc("repo_id").Limit(batchSize, start).
			GroupBy("repo_id").Having("COUNT(*) > 25").Find(&repoTopics); err != nil {
			return err
		}
		if len(repoTopics) == 0 {
			break
		}

		log.Info("Number of repositories with more than 25 topics: %v", len(repoTopics))
		for _, repoTopic := range repoTopics {
			touchedRepo[repoTopic.RepoID] = struct{}{}

			tmpRepoTopics := make([]*RepoTopic, 0, 30)
			if err := x.Where("repo_id = ?", repoTopic.RepoID).Find(&tmpRepoTopics); err != nil {
				return err
			}

			log.Info("Repository with id = %v has %v topics", repoTopic.RepoID, len(tmpRepoTopics))

			for i := len(tmpRepoTopics) - 1; i > 24; i-- {
				delRepoTopics = append(delRepoTopics, tmpRepoTopics[i])
			}
		}
	}

	sess.Init()

	log.Info("Deleting superfluous topics for repositories (more than 25 topics)...")
	if err := sess.Begin(); err != nil {
		return err
	}
	for _, repoTopic := range delRepoTopics {
		log.Info("Deleting 'repo_topic' rows for 'repository' with id = %v. Topic id = %v",
			repoTopic.RepoID, repoTopic.TopicID)

		if _, err := sess.Where("repo_id = ? AND topic_id = ?", repoTopic.RepoID,
			repoTopic.TopicID).Delete(&RepoTopic{}); err != nil {
			return err
		}
		if _, err := sess.Exec(
			"UPDATE topic SET repo_count = (SELECT repo_count FROM topic WHERE id = ?) - 1 WHERE id = ?",
			repoTopic.TopicID, repoTopic.TopicID); err != nil {
			return err
		}
	}

	log.Info("Updating repositories 'topics' fields...")
	for repoID := range touchedRepo {
		topicNames := make([]string, 0, 30)
		if err := sess.Table("topic").Cols("name").
			Join("INNER", "repo_topic", "repo_topic.topic_id = topic.id").
			Where("repo_topic.repo_id = ?", repoID).Desc("topic.repo_count").Find(&topicNames); err != nil {
			return err
		}
		log.Info("Updating 'topics' field for repository with id = %v", repoID)
		if _, err := sess.ID(repoID).Cols("topics").
			Update(&Repository{Topics: topicNames}); err != nil {
			return err
		}
	}

	return sess.Commit()
}
