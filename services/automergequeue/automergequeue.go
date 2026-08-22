// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automergequeue

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/log"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/setting"
)

type AutoMergeItem string // it is a unique queue, so the item type can't be JSON which doesn't have deterministic key order.

var AutoMergeQueue *queue.WorkerPoolQueue[AutoMergeItem]

func (item AutoMergeItem) Parse() (ret struct {
	PullID int64

	RepoID   int64
	CommitID string
},
) {
	typ, remaining, _ := strings.Cut(string(item), ":")
	args := strings.Split(remaining, ",")
	switch typ {
	case "pr":
		ret.PullID, _ = strconv.ParseInt(args[0], 10, 64)
	case "repo-commit":
		ret.RepoID, _ = strconv.ParseInt(args[0], 10, 64)
		ret.CommitID = args[1]
	default:
		if setting.IsProd || setting.IsInTesting {
			panic("invalid auto merge item type")
		}
	}
	return ret
}

var AddToQueue = func(item AutoMergeItem) {
	if err := AutoMergeQueue.Push(item); err != nil && !errors.Is(err, queue.ErrAlreadyInQueue) {
		log.Error("Error adding %v to the automerge queue: %v", item, err)
	}
}

func StartPRCheckAndAutoMerge(pull *issues_model.PullRequest) {
	AddToQueue(AutoMergeItem(fmt.Sprintf("pr:%d", pull.ID)))
}

func StartPRCheckAndAutoMergeByCommit(repoID int64, commitID string) {
	AddToQueue(AutoMergeItem(fmt.Sprintf("repo-commit:%d,%s", repoID, commitID)))
}
