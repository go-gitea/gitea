// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(Topic))
	db.RegisterModel(new(RepoTopic))
}

var topicPattern = regexp.MustCompile(`^[a-z0-9][-.a-z0-9]*$`)

// Topic represents a topic of repositories
type Topic struct {
	ID          int64  `xorm:"pk autoincr"`
	Name        string `xorm:"UNIQUE VARCHAR(50)"`
	RepoCount   int
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// RepoTopic represents associated repositories and topics
type RepoTopic struct { //revive:disable-line:exported
	RepoID  int64 `xorm:"pk"`
	TopicID int64 `xorm:"pk"`
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

func (err ErrTopicNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ValidateTopic checks a topic by length and match pattern rules
func ValidateTopic(topic string) bool {
	return len(topic) <= 35 && topicPattern.MatchString(topic)
}

// SanitizeAndValidateTopics sanitizes and checks an array or topics
func SanitizeAndValidateTopics(topics []string) (validTopics, invalidTopics []string) {
	validTopics = make([]string, 0)
	mValidTopics := make(container.Set[string])
	invalidTopics = make([]string, 0)

	for _, topic := range topics {
		topic = strings.TrimSpace(strings.ToLower(topic))
		// ignore empty string
		if len(topic) == 0 {
			continue
		}
		// ignore same topic twice
		if mValidTopics.Contains(topic) {
			continue
		}
		if ValidateTopic(topic) {
			validTopics = append(validTopics, topic)
			mValidTopics.Add(topic)
		} else {
			invalidTopics = append(invalidTopics, topic)
		}
	}

	return validTopics, invalidTopics
}

// GetTopicByName retrieves topic by name
func GetTopicByName(ctx context.Context, name string) (*Topic, error) {
	var topic Topic
	if has, err := db.GetEngine(ctx).Where("name = ?", name).Get(&topic); err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTopicNotExist{name}
	}
	return &topic, nil
}

// addTopicByNameToRepo adds a topic name to a repo and increments the topic count.
// Returns topic after the addition
func addTopicByNameToRepo(ctx context.Context, repoID int64, topicName string) (*Topic, error) {
	var topic Topic
	e := db.GetEngine(ctx)
	has, err := e.Where("name = ?", topicName).Get(&topic)
	if err != nil {
		return nil, err
	}
	if !has {
		topic.Name = topicName
		topic.RepoCount = 1
		if err := db.Insert(ctx, &topic); err != nil {
			return nil, err
		}
	} else {
		topic.RepoCount++
		if _, err := e.ID(topic.ID).Cols("repo_count").Update(&topic); err != nil {
			return nil, err
		}
	}

	if err := db.Insert(ctx, &RepoTopic{
		RepoID:  repoID,
		TopicID: topic.ID,
	}); err != nil {
		return nil, err
	}

	return &topic, nil
}

// removeTopicFromRepo remove a topic from a repo and decrements the topic repo count
func removeTopicFromRepo(ctx context.Context, repoID int64, topic *Topic) error {
	topic.RepoCount--
	e := db.GetEngine(ctx)
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

// RemoveTopicsFromRepo remove all topics from the repo and decrements respective topics repo count
func RemoveTopicsFromRepo(ctx context.Context, repoID int64) error {
	e := db.GetEngine(ctx)
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
	db.ListOptions
	RepoID  int64
	Keyword string
}

func (opts *FindTopicOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_topic.repo_id": opts.RepoID})
	}

	if opts.Keyword != "" {
		cond = cond.And(builder.Like{"topic.name", opts.Keyword})
	}

	return cond
}

func (opts *FindTopicOptions) ToOrders() string {
	orderBy := "topic.repo_count DESC"
	if opts.RepoID > 0 {
		orderBy = "topic.name" // when render topics for a repo, it's better to sort them by name, to get consistent result
	}
	return orderBy
}

func (opts *FindTopicOptions) ToJoins() []db.JoinFunc {
	if opts.RepoID <= 0 {
		return nil
	}
	return []db.JoinFunc{
		func(e db.Engine) error {
			e.Join("INNER", "repo_topic", "repo_topic.topic_id = topic.id")
			return nil
		},
	}
}

// GetRepoTopicByName retrieves topic from name for a repo if it exist
func GetRepoTopicByName(ctx context.Context, repoID int64, topicName string) (*Topic, error) {
	cond := builder.NewCond()
	var topic Topic
	cond = cond.And(builder.Eq{"repo_topic.repo_id": repoID}).And(builder.Eq{"topic.name": topicName})
	sess := db.GetEngine(ctx).Table("topic").Where(cond)
	sess.Join("INNER", "repo_topic", "repo_topic.topic_id = topic.id")
	has, err := sess.Select("topic.*").Get(&topic)
	if has {
		return &topic, err
	}
	return nil, err
}

// AddTopic adds a topic name to a repository (if it does not already have it)
func AddTopic(ctx context.Context, repoID int64, topicName string) (*Topic, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	topic, err := GetRepoTopicByName(ctx, repoID, topicName)
	if err != nil {
		return nil, err
	}
	if topic != nil {
		// Repo already have topic
		return topic, nil
	}

	topic, err = addTopicByNameToRepo(ctx, repoID, topicName)
	if err != nil {
		return nil, err
	}

	if err = syncTopicsInRepository(sess, repoID); err != nil {
		return nil, err
	}

	return topic, committer.Commit()
}

// DeleteTopic removes a topic name from a repository (if it has it)
func DeleteTopic(ctx context.Context, repoID int64, topicName string) (*Topic, error) {
	topic, err := GetRepoTopicByName(ctx, repoID, topicName)
	if err != nil {
		return nil, err
	}
	if topic == nil {
		// Repo doesn't have topic, can't be removed
		return nil, nil
	}

	err = removeTopicFromRepo(ctx, repoID, topic)
	if err != nil {
		return nil, err
	}

	err = syncTopicsInRepository(db.GetEngine(ctx), repoID)

	return topic, err
}

// SaveTopics save topics to a repository
func SaveTopics(ctx context.Context, repoID int64, topicNames ...string) error {
	topics, err := db.Find[Topic](ctx, &FindTopicOptions{
		RepoID: repoID,
	})
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

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
		_, err := addTopicByNameToRepo(ctx, repoID, topicName)
		if err != nil {
			return err
		}
	}

	for _, topic := range removeTopics {
		err := removeTopicFromRepo(ctx, repoID, topic)
		if err != nil {
			return err
		}
	}

	if err := syncTopicsInRepository(sess, repoID); err != nil {
		return err
	}

	return committer.Commit()
}

// GenerateTopics generates topics from a template repository
func GenerateTopics(ctx context.Context, templateRepo, generateRepo *Repository) error {
	for _, topic := range templateRepo.Topics {
		if _, err := addTopicByNameToRepo(ctx, generateRepo.ID, topic); err != nil {
			return err
		}
	}

	return syncTopicsInRepository(db.GetEngine(ctx), generateRepo.ID)
}

// syncTopicsInRepository makes sure topics in the topics table are copied into the topics field of the repository
func syncTopicsInRepository(sess db.Engine, repoID int64) error {
	topicNames := make([]string, 0, 25)
	if err := sess.Table("topic").Cols("name").
		Join("INNER", "repo_topic", "repo_topic.topic_id = topic.id").
		Where("repo_topic.repo_id = ?", repoID).Asc("topic.name").Find(&topicNames); err != nil {
		return err
	}

	if _, err := sess.ID(repoID).Cols("topics").Update(&Repository{
		Topics: topicNames,
	}); err != nil {
		return err
	}
	return nil
}

// CountOrphanedAttachments returns the number of topics that don't belong to any repository.
func CountOrphanedTopics(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Where("repo_count = 0").Count(new(Topic))
}

// DeleteOrphanedAttachments delete all topics that don't belong to any repository.
func DeleteOrphanedTopics(ctx context.Context) (int64, error) {
	return db.GetEngine(ctx).Where("repo_count = 0").Delete(new(Topic))
}
