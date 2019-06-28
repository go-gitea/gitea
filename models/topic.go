// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

func init() {
	tables = append(tables,
		new(Topic),
		new(RepoTopic),
	)
}

var topicPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Topic represents a topic of repositories
type Topic struct {
	ID          int64
	Name        string `xorm:"UNIQUE VARCHAR(25)"`
	RepoCount   int
	CreatedUnix util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`
}

// RepoTopic represents associated repositories and topics
type RepoTopic struct {
	RepoID  int64 `xorm:"UNIQUE(s)"`
	TopicID int64 `xorm:"UNIQUE(s)"`
}

// ErrTopicNotExist represents an error that a topic is not exist
type ErrTopicNotExist struct {
	Name string
}

// IsErrTopicNotExist checks if an error is an ErrTopicNotExist.
func IsErrTopicNotExist(err error) bool {
	_, ok := err.(ErrTopicNotExist)
	return ok
}

// Error implements error interface
func (err ErrTopicNotExist) Error() string {
	return fmt.Sprintf("topic is not exist [name: %s]", err.Name)
}

// ValidateTopic checks topics by length and match pattern rules
func ValidateTopic(topic string) bool {
	return len(topic) <= 35 && topicPattern.MatchString(topic)
}

// GetTopicByName retrieves topic by name
func GetTopicByName(name string) (*Topic, error) {
	var topic Topic
	if has, err := x.Where("name = ?", name).Get(&topic); err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTopicNotExist{name}
	}
	return &topic, nil
}

// FindTopicOptions represents the options when fdin topics
type FindTopicOptions struct {
	RepoID  int64
	Keyword string
	Limit   int
	Page    int
}

func (opts *FindTopicOptions) toConds() builder.Cond {
	var cond = builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_topic.repo_id": opts.RepoID})
	}

	if opts.Keyword != "" {
		cond = cond.And(builder.Like{"topic.name", opts.Keyword})
	}

	return cond
}

// FindTopics retrieves the topics via FindTopicOptions
func FindTopics(opts *FindTopicOptions) (topics []*Topic, err error) {
	sess := x.Select("topic.*").Where(opts.toConds())
	if opts.RepoID > 0 {
		sess.Join("INNER", "repo_topic", "repo_topic.topic_id = topic.id")
	}
	if opts.Limit > 0 {
		sess.Limit(opts.Limit, opts.Page*opts.Limit)
	}
	return topics, sess.Desc("topic.repo_count").Find(&topics)
}

// SaveTopics save topics to a repository
func SaveTopics(repoID int64, topicNames ...string) error {
	topics, err := FindTopics(&FindTopicOptions{
		RepoID: repoID,
	})
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	var addedTopicNames []string
	for _, topicName := range topicNames {
		if strings.TrimSpace(topicName) == "" {
			continue
		}

		var found bool
		for _, t := range topics {
			if strings.EqualFold(topicName, t.Name) {
				found = true
				break
			}
		}
		if !found {
			addedTopicNames = append(addedTopicNames, topicName)
		}
	}

	var removeTopics []*Topic
	for _, t := range topics {
		var found bool
		for _, topicName := range topicNames {
			if strings.EqualFold(topicName, t.Name) {
				found = true
				break
			}
		}
		if !found {
			removeTopics = append(removeTopics, t)
		}
	}

	for _, topicName := range addedTopicNames {
		var topic Topic
		if has, err := sess.Where("name = ?", topicName).Get(&topic); err != nil {
			return err
		} else if !has {
			topic.Name = topicName
			topic.RepoCount = 1
			if _, err := sess.Insert(&topic); err != nil {
				return err
			}
		} else {
			topic.RepoCount++
			if _, err := sess.ID(topic.ID).Cols("repo_count").Update(&topic); err != nil {
				return err
			}
		}

		if _, err := sess.Insert(&RepoTopic{
			RepoID:  repoID,
			TopicID: topic.ID,
		}); err != nil {
			return err
		}
	}

	for _, topic := range removeTopics {
		topic.RepoCount--
		if _, err := sess.ID(topic.ID).Cols("repo_count").Update(topic); err != nil {
			return err
		}

		if _, err := sess.Delete(&RepoTopic{
			RepoID:  repoID,
			TopicID: topic.ID,
		}); err != nil {
			return err
		}
	}

	topicNames = make([]string, 0, 25)
	if err := sess.Table("topic").Cols("name").
		Join("INNER", "repo_topic", "repo_topic.topic_id = topic.id").
		Where("repo_topic.repo_id = ?", repoID).Desc("topic.repo_count").Find(&topicNames); err != nil {
		return err
	}

	if _, err := sess.ID(repoID).Cols("topics").Update(&Repository{
		Topics: topicNames,
	}); err != nil {
		return err
	}

	return sess.Commit()
}
