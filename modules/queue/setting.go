// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package queue

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// UniqueQueueName represents an expected name for an UniqueQueue
type UniqueQueueName string

// list of all expected UniqueQueues
const (
	CodeIndexerQueueName     UniqueQueueName = "code_indexer"
	RepoStatsUpdateQueueName UniqueQueueName = "repo_stats_update"
	MirrorQueueName          UniqueQueueName = "mirror"
	PRPatchQueueName         UniqueQueueName = "pr_patch_checker"
	RepoArchiveQueueName     UniqueQueueName = "repo-archive"
	PRAutoMergeQueueName     UniqueQueueName = "pr_auto_merge"
	WebhookDeliveryQueueName UniqueQueueName = "webhook_sender"
)

// KnownUniqueQueueNames represents the list of expected unique queues
var KnownUniqueQueueNames = []UniqueQueueName{
	CodeIndexerQueueName,
	RepoStatsUpdateQueueName,
	MirrorQueueName,
	PRPatchQueueName,
	RepoArchiveQueueName,
}

// QueueName represents an expected name for Queue
type QueueName string // nolint // allow this to stutter

// list of all expected Queues
const (
	IssueIndexerQueueName QueueName = "issue_indexer"
	NotificationQueueName QueueName = "notification-service"
	MailerQueueName       QueueName = "mail"
	PushUpdateQueueName   QueueName = "push_update"
	TaskQueueName         QueueName = "task"
)

// KnownQueueNames represents the list of expected queues
var KnownQueueNames = []QueueName{
	IssueIndexerQueueName,
	NotificationQueueName,
	MailerQueueName,
	PushUpdateQueueName,
	TaskQueueName,
}

func validType(t string) (Type, error) {
	if len(t) == 0 {
		return PersistableChannelQueueType, nil
	}
	for _, typ := range RegisteredTypes() {
		if t == string(typ) {
			return typ, nil
		}
	}
	return PersistableChannelQueueType, fmt.Errorf("unknown queue type: %s defaulting to %s", t, string(PersistableChannelQueueType))
}

func getQueueSettings(name string) (setting.QueueSettings, []byte) {
	q := setting.GetQueueSettings(name)
	cfg, err := json.Marshal(q)
	if err != nil {
		log.Error("Unable to marshall generic options: %v Error: %v", q, err)
		log.Error("Unable to create queue for %s", name, err)
		return q, []byte{}
	}
	return q, cfg
}

// CreateQueue for name with provided handler and exemplar
func CreateQueue(name QueueName, handle HandlerFunc, exemplar interface{}) Queue {
	found := false
	for _, expected := range KnownQueueNames {
		if name == expected {
			found = true
			break
		}
	}
	if !found {
		log.Warn("%s is not an expected name for an Queue", name)
	}

	q, cfg := getQueueSettings(string(name))
	if len(cfg) == 0 {
		return nil
	}

	typ, err := validType(q.Type)
	if err != nil {
		log.Error("Invalid type %s provided for queue named %s defaulting to %s", q.Type, name, string(typ))
	}

	returnable, err := NewQueue(typ, handle, cfg, exemplar)
	if q.WrapIfNecessary && err != nil {
		log.Warn("Unable to create queue for %s: %v", name, err)
		log.Warn("Attempting to create wrapped queue")
		returnable, err = NewQueue(WrappedQueueType, handle, WrappedQueueConfiguration{
			Underlying:  typ,
			Timeout:     q.Timeout,
			MaxAttempts: q.MaxAttempts,
			Config:      cfg,
			QueueLength: q.QueueLength,
			Name:        string(name),
		}, exemplar)
	}
	if err != nil {
		log.Error("Unable to create queue for %s: %v", name, err)
		return nil
	}

	// Sanity check configuration
	if q.Workers == 0 && (q.BoostTimeout == 0 || q.BoostWorkers == 0 || q.MaxWorkers == 0) {
		log.Warn("Queue: %s is configured to be non-scaling and have no workers\n - this configuration is likely incorrect and could cause Gitea to block", q.Name)
		if pausable, ok := returnable.(Pausable); ok {
			log.Warn("Queue: %s is being paused to prevent data-loss, add workers manually and unpause.", q.Name)
			pausable.Pause()
		}
	}

	return returnable
}

// CreateUniqueQueue for name with provided handler and exemplar
func CreateUniqueQueue(name UniqueQueueName, handle HandlerFunc, exemplar interface{}) UniqueQueue {
	found := false
	for _, expected := range KnownUniqueQueueNames {
		if name == expected {
			found = true
			break
		}
	}
	if !found {
		log.Warn("%s is not an expected name for an UniqueQueue", name)
	}

	q, cfg := getQueueSettings(string(name))
	if len(cfg) == 0 {
		return nil
	}

	if len(q.Type) > 0 && q.Type != "dummy" && q.Type != "immediate" && !strings.HasPrefix(q.Type, "unique-") {
		q.Type = "unique-" + q.Type
	}

	typ, err := validType(q.Type)
	if err != nil || typ == PersistableChannelQueueType {
		typ = PersistableChannelUniqueQueueType
		if err != nil {
			log.Error("Invalid type %s provided for queue named %s defaulting to %s", q.Type, name, string(typ))
		}
	}

	returnable, err := NewQueue(typ, handle, cfg, exemplar)
	if q.WrapIfNecessary && err != nil {
		log.Warn("Unable to create unique queue for %s: %v", name, err)
		log.Warn("Attempting to create wrapped queue")
		returnable, err = NewQueue(WrappedUniqueQueueType, handle, WrappedUniqueQueueConfiguration{
			Underlying:  typ,
			Timeout:     q.Timeout,
			MaxAttempts: q.MaxAttempts,
			Config:      cfg,
			QueueLength: q.QueueLength,
			Name:        string(name),
		}, exemplar)
	}
	if err != nil {
		log.Error("Unable to create unique queue for %s: %v", name, err)
		return nil
	}

	// Sanity check configuration
	if q.Workers == 0 && (q.BoostTimeout == 0 || q.BoostWorkers == 0 || q.MaxWorkers == 0) {
		log.Warn("Queue: %s is configured to be non-scaling and have no workers\n - this configuration is likely incorrect and could cause Gitea to block", q.Name)
		if pausable, ok := returnable.(Pausable); ok {
			log.Warn("Queue: %s is being paused to prevent data-loss, add workers manually and unpause.", q.Name)
			pausable.Pause()
		}
	}

	return returnable.(UniqueQueue)
}
