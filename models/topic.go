// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/timeutil"

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
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
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

// ValidateTopic checks a topic by length and match pattern rules
func ValidateTopic(topic string) bool {
	return len(topic) <= 35 && topicPattern.MatchString(topic)
}

// SanitizeAndValidateTopics sanitizes and checks an array or topics
func SanitizeAndValidateTopics(topics []string) (validTopics []string, invalidTopics []string) {
	validTopics = make([]string, 0)
	mValidTopics := make(map[string]struct{})
	invalidTopics = make([]string, 0)

	for _, topic := range topics {
		topic = strings.TrimSpace(strings.ToLower(topic))
		// ignore empty string
		if len(topic) == 0 {
			continue
		}
		// ignore same topic twice
		if _, ok := mValidTopics[topic]; ok {
			continue
		}
		if ValidateTopic(topic) {
			validTopics = append(validTopics, topic)
			mValidTopics[topic] = struct{}{}
		} else {
			invalidTopics = append(invalidTopics, topic)
		}
	}

	return validTopics, invalidTopics
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

// addTopicByNameToRepo adds a topic name to a repo and increments the topic count.
// Returns topic after the addition
func addTopicByNameToRepo(e Engine, repoID int64, topicName string) (*Topic, error) {
	var topic Topic
	has, err := e.Where("name = ?", topicName).Get(&topic)
	if err != nil {
		return nil, err
	}
	if !has {
		topic.Name = topicName
		topic.RepoCount = 1
		if _, err := e.Insert(&topic); err != nil {
			return nil, err
		}
	} else {
		topic.RepoCount++
		if _, err := e.ID(topic.ID).Cols("repo_count").Update(&topic); err != nil {
			return nil, err
		}
	}

	if _, err := e.Insert(&RepoTopic{
		RepoID:  repoID,
		TopicID: topic.ID,
	}); err != nil {
		return nil, err
	}

	return &topic, nil
}

// removeTopicFromRepo remove a topic from a repo and decrements the topic repo count
func removeTopicFromRepo(e Engine, repoID int64, topic *Topic) error {
	topic.RepoCount--
	if _, err := e.ID(topic.ID).Cols("repo_count").Update(topic); err != nil {
		return err
	}

	if _, err := e.Delete(&RepoTopic{
		RepoID:  repoID,
		TopicID: topic.ID,
	}); err != nil {
		return err
	}

	return nil
}

// removeTopicsFromRepo remove all topics from the repo and decrements respective topics repo count
func removeTopicsFromRepo(e Engine, repoID int64) error {
	_, err := e.Where(
		builder.In("id",
			builder.Select("topic_id").From("repo_topic").Where(builder.Eq{"repo_id": repoID}),
		),
	).Cols("repo_count").SetExpr("repo_count", "repo_count-1").Update(&Topic{})
	if err != nil {
		return err
	}

	if _, err = e.Delete(&RepoTopic{RepoID: repoID}); err != nil {
		return err
	}

	return nil
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

// GetRepoTopicByName retrives topic from name for a repo if it exist
func GetRepoTopicByName(repoID int64, topicName string) (*Topic, error) {
	var cond = builder.NewCond()
	var topic Topic
	cond = cond.And(builder.Eq{"repo_topic.repo_id": repoID}).And(builder.Eq{"topic.name": topicName})
	sess := x.Table("topic").Where(cond)
	sess.Join("INNER", "repo_topic", "repo_topic.topic_id = topic.id")
	has, err := sess.Get(&topic)
	if has {
		return &topic, err
	}
	return nil, err
}

// AddTopic adds a topic name to a repository (if it does not already have it)
func AddTopic(repoID int64, topicName string) (*Topic, error) {
	topic, err := GetRepoTopicByName(repoID, topicName)
	if err != nil {
		return nil, err
	}
	if topic != nil {
		// Repo already have topic
		return topic, nil
	}

	return addTopicByNameToRepo(x, repoID, topicName)
}

// DeleteTopic removes a topic name from a repository (if it has it)
func DeleteTopic(repoID int64, topicName string) (*Topic, error) {
	topic, err := GetRepoTopicByName(repoID, topicName)
	if err != nil {
		return nil, err
	}
	if topic == nil {
		// Repo doesn't have topic, can't be removed
		return nil, nil
	}

	err = removeTopicFromRepo(x, repoID, topic)

	return topic, err
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
		_, err := addTopicByNameToRepo(sess, repoID, topicName)
		if err != nil {
			return err
		}
	}

	for _, topic := range removeTopics {
		err := removeTopicFromRepo(sess, repoID, topic)
		if err != nil {
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
