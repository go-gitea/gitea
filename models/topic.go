// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/util"

	"github.com/go-xorm/builder"
)

func init() {
	tables = append(tables,
		new(Topic),
		new(RepoTopic),
	)
}

// Topic represents a topic of repositories
type Topic struct {
	ID          int64
	Name        string `xorm:"unique"`
	RepoCount   int
	CreatedUnix util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`
}

// RepositoryTopic represents associated repositories and topics
type RepoTopic struct {
	RepoID  int64 `xorm:"unique(s)"`
	TopicID int64 `xorm:"unique(s)"`
}

// ErrTopicNotExist represents an error that a topic is not exist
type ErrTopicNotExist struct {
	Name string
}

// IsErrNotAllowedToMerge checks if an error is an ErrTopicNotExist.
func IsErrTopicNotExist(err error) bool {
	_, ok := err.(ErrTopicNotExist)
	return ok
}

func (err ErrTopicNotExist) Error() string {
	return fmt.Sprintf("topic is not exist [name: %s]", err.Name)
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
	RepoID int64
	Limit  int
	Page   int
}

func (opts *FindTopicOptions) toConds() builder.Cond {
	var cond = builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_topic.repo_id": opts.RepoID})
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
	return topics, sess.Find(&topics)
}

// AddTopic addes a topic to a repository
func AddTopic(repoID int64, topicName string) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

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

	return sess.Commit()
}

// RemoveTopicFromRepo removes topic from a repoisotry
func RemoveTopicFromRepo(repoID int64, topicName string) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	var topic Topic
	if has, err := sess.Where("name = ?", topicName).Get(&topic); err != nil {
		return err
	} else if !has {
		return ErrTopicNotExist{topicName}
	}

	if _, err := sess.Delete(&RepoTopic{
		RepoID:  repoID,
		TopicID: topic.ID,
	}); err != nil {
		return err
	}

	if _, err := sess.ID(topic.ID).Decr("repo_count").Cols("repo_count").Update(&topic); err != nil {
		return err
	}

	return sess.Commit()
}
