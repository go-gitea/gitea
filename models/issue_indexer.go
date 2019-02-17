// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var (
	// issueIndexerUpdateQueue queue of issue ids to be updated
	issueIndexerUpdateQueue issues.Queue
	issueIndexer            issues.Indexer
)

// InitIssueIndexer initialize issue indexer
func InitIssueIndexer() error {
	var populate bool
	switch setting.Indexer.IssueType {
	case "bleve":
		issueIndexer = issues.NewBleveIndexer(setting.Indexer.IssuePath)
		exist, err := issueIndexer.Init()
		if err != nil {
			return err
		}
		populate = !exist
	default:
		return fmt.Errorf("unknow issue indexer type: %s", setting.Indexer.IssueType)
	}

	var err error
	switch setting.Indexer.IssueIndexerQueueType {
	case setting.LevelQueueType:
		issueIndexerUpdateQueue, err = issues.NewLevelQueue(
			issueIndexer,
			setting.Indexer.IssueIndexerQueueDir,
			setting.Indexer.IssueIndexerQueueBatchNumber)
		if err != nil {
			return err
		}
	case setting.ChannelQueueType:
		issueIndexerUpdateQueue = issues.NewChannelQueue(issueIndexer, setting.Indexer.IssueIndexerQueueBatchNumber)
	default:
		return fmt.Errorf("Unsupported indexer queue type: %v", setting.Indexer.IssueIndexerQueueType)
	}

	go issueIndexerUpdateQueue.Run()

	if populate {
		go populateIssueIndexer()
	}

	return nil
}

// populateIssueIndexer populate the issue indexer with issue data
func populateIssueIndexer() {
	for page := 1; ; page++ {
		repos, _, err := SearchRepositoryByName(&SearchRepoOptions{
			Page:        page,
			PageSize:    RepositoryListDefaultPageSize,
			OrderBy:     SearchOrderByID,
			Private:     true,
			Collaborate: util.OptionalBoolFalse,
		})
		if err != nil {
			log.Error(4, "SearchRepositoryByName: %v", err)
			continue
		}
		if len(repos) == 0 {
			return
		}

		for _, repo := range repos {
			is, err := Issues(&IssuesOptions{
				RepoIDs:  []int64{repo.ID},
				IsClosed: util.OptionalBoolNone,
				IsPull:   util.OptionalBoolNone,
			})
			if err != nil {
				log.Error(4, "Issues: %v", err)
				continue
			}
			if err = IssueList(is).LoadDiscussComments(); err != nil {
				log.Error(4, "LoadComments: %v", err)
				continue
			}
			for _, issue := range is {
				UpdateIssueIndexer(issue)
			}
		}
	}
}

// UpdateIssueIndexer add/update an issue to the issue indexer
func UpdateIssueIndexer(issue *Issue) {
	var comments []string
	for _, comment := range issue.Comments {
		if comment.Type == CommentTypeComment {
			comments = append(comments, comment.Content)
		}
	}
	issueIndexerUpdateQueue.Push(&issues.IndexerData{
		ID:       issue.ID,
		RepoID:   issue.RepoID,
		Title:    issue.Title,
		Content:  issue.Content,
		Comments: comments,
	})
}

// DeleteRepoIssueIndexer deletes repo's all issues indexes
func DeleteRepoIssueIndexer(repo *Repository) {
	var ids []int64
	ids, err := getIssueIDsByRepoID(x, repo.ID)
	if err != nil {
		log.Error(4, "getIssueIDsByRepoID failed: %v", err)
		return
	}

	if len(ids) <= 0 {
		return
	}

	issueIndexerUpdateQueue.Push(&issues.IndexerData{
		IDs:      ids,
		IsDelete: true,
	})
}

// SearchIssuesByKeyword search issue ids by keywords and repo id
func SearchIssuesByKeyword(repoID int64, keyword string) ([]int64, error) {
	var issueIDs []int64
	res, err := issueIndexer.Search(keyword, repoID, 1000, 0)
	if err != nil {
		return nil, err
	}
	for _, r := range res.Hits {
		issueIDs = append(issueIDs, r.ID)
	}
	return issueIDs, nil
}
