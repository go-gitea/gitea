// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// issueIndexerUpdateQueue queue of issue ids to be updated
var issueIndexerUpdateQueue chan int64

// InitIssueIndexer initialize issue indexer
func InitIssueIndexer() {
	indexer.InitIssueIndexer(populateIssueIndexer)
	issueIndexerUpdateQueue = make(chan int64, setting.Indexer.UpdateQueueLength)
	go processIssueIndexerUpdateQueue()
}

// populateIssueIndexer populate the issue indexer with issue data
func populateIssueIndexer() error {
	for page := 1; ; page++ {
		repos, _, err := Repositories(&SearchRepoOptions{
			Page:     page,
			PageSize: 10,
		})
		if err != nil {
			return fmt.Errorf("Repositories: %v", err)
		}
		if len(repos) == 0 {
			return nil
		}
		for _, repo := range repos {
			issues, err := Issues(&IssuesOptions{
				RepoID:   repo.ID,
				IsClosed: util.OptionalBoolNone,
				IsPull:   util.OptionalBoolNone,
			})
			updates := make([]indexer.IssueIndexerUpdate, len(issues))
			for i, issue := range issues {
				updates[i] = issue.update()
			}
			if err = indexer.BatchUpdateIssues(updates...); err != nil {
				return fmt.Errorf("BatchUpdate: %v", err)
			}
		}
	}
}

func processIssueIndexerUpdateQueue() {
	for {
		select {
		case issueID := <-issueIndexerUpdateQueue:
			issue, err := GetIssueByID(issueID)
			if err != nil {
				log.Error(4, "issuesIndexer.Index: %v", err)
				continue
			}
			if err = indexer.UpdateIssue(issue.update()); err != nil {
				log.Error(4, "issuesIndexer.Index: %v", err)
			}
		}
	}
}

func (issue *Issue) update() indexer.IssueIndexerUpdate {
	comments := make([]string, 0, 5)
	for _, comment := range issue.Comments {
		if comment.Type == CommentTypeComment {
			comments = append(comments, comment.Content)
		}
	}
	return indexer.IssueIndexerUpdate{
		IssueID: issue.ID,
		Data: &indexer.IssueIndexerData{
			RepoID:   issue.RepoID,
			Title:    issue.Title,
			Content:  issue.Content,
			Comments: comments,
		},
	}
}

// UpdateIssueIndexer add/update an issue to the issue indexer
func UpdateIssueIndexer(issueID int64) {
	select {
	case issueIndexerUpdateQueue <- issueID:
	default:
		go func() {
			issueIndexerUpdateQueue <- issueID
		}()
	}
}
