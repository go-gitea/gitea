// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/log"
	"xorm.io/builder"
)

// Cleanup removes closed issues from project board with close on move flag set to true
func CleanupProjectIssues(taskCtx context.Context, olderThan time.Duration) error {
	log.Info("CRON:  remove closed issues from boards with close on move flag set to  true starting...")

	projects, err := getAllProjects(taskCtx)

	if err != nil {
		return err
	}
	for _, project := range projects {
		boards, err := getAllProjectBoardWithCloseOnMove(taskCtx, project)
		if err != nil {
			log.Error("Cannot get boards  of project ID %d: %v", project.ID, err)
			continue
		}
		log.Info("Found %d boards with close on move true", len(boards))
		for _, board := range boards {
			issues, err := getAllIssuesOfBoard(taskCtx, board)
			if err != nil {
				log.Error("Cannot get issues of board ID %d: %v", board.ID, err)
				continue
			}
			issuesToBeRemoved, err := getAllIssuesToBeRemoved(taskCtx, issues)
			if err != nil {
				log.Error("Cannot get issues of to be removed of board ID %d: %v", board.ID, err)
				continue
			}
			for _, issueToBeRemoved := range issuesToBeRemoved {
				err = removeIssueFromProject(taskCtx, issueToBeRemoved, project)
				if err != nil {
					log.Error("Cannot remove issue ID %d from board ID %d: %v", issueToBeRemoved.ID, board.ID, err)
					continue
				}
				log.Info("Removed issue ID %d from board ID %d", issueToBeRemoved.ID, board.ID)
			}
			log.Info("completed removing closed issues from board ID %d", board.ID)
		}
		log.Info("completed removing closed issues  project ID %d", project.ID)
	}

	log.Info("CRON:  remove closed issues from boards with close on move flag true completed.")

	return nil
}

func getAllProjects(ctx context.Context) ([]project_model.Project, error) {

	var projects []project_model.Project

	err := db.GetEngine(ctx).Table("project").Select("*").Find(&projects)
	if err != nil {
		fmt.Println(err)
		return projects, err
	}
	return projects, nil
}

func getAllProjectBoardWithCloseOnMove(ctx context.Context, project project_model.Project) ([]project_model.Board, error) {
	var boards []project_model.Board

	err := db.GetEngine(ctx).Table("project_board").Select("*").Where(builder.Eq{"project_id": project.ID}).Find(&boards)
	if err != nil {
		fmt.Println(err)
		return boards, err
	}
	return boards, nil
}

func getAllIssuesOfBoard(ctx context.Context, board project_model.Board) ([]int64, error) {
	var issueIDs []int64

	err := db.GetEngine(ctx).Table("project_issue").Select("issue_id").Where(builder.Eq{"project_id": board.ProjectID, "project_board_id": board.ID}).Find(&issueIDs)
	if err != nil {
		fmt.Println(err)
		return issueIDs, err
	}
	return issueIDs, nil
}

func getAllIssuesToBeRemoved(ctx context.Context, issueIDs []int64) ([]issues_model.Issue, error) {

	var issues []issues_model.Issue

	err := db.GetEngine(ctx).Table("issue").Select("*").Where(builder.Eq{"is_closed": 1}).Where(builder.In("id", issueIDs)).Find(&issues)
	if err != nil {
		fmt.Println(err)
		return issues, err
	}

	return issues, nil
}

func removeIssueFromProject(ctx context.Context, issue issues_model.Issue, project project_model.Project) error {

	project_issue := &project_model.ProjectIssue{
		IssueID:   issue.ID,
		ProjectID: project.ID,
	}

	_, err := db.GetEngine(ctx).Table("project_issue").Delete(&project_issue)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil

}
