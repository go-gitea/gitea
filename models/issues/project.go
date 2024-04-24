// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"errors"
	"sort"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
)

type ProjectMovedIssuesFormItem struct {
	IssueID int64 `json:"issueID"`
	Sorting int64 `json:"sorting"`
}

type ProjectMovedIssuesForm struct {
	Issues []ProjectMovedIssuesFormItem `json:"issues"`
}

func (p *ProjectMovedIssuesForm) ToSortedIssueIDs() (issueIDs, issueSorts []int64) {
	sort.Slice(p.Issues, func(i, j int) bool { return p.Issues[i].Sorting < p.Issues[j].Sorting })

	issueIDs = make([]int64, 0, len(p.Issues))
	issueSorts = make([]int64, 0, len(p.Issues))

	for _, issue := range p.Issues {
		issueIDs = append(issueIDs, issue.IssueID)
		issueSorts = append(issueSorts, issue.Sorting)
	}

	return issueIDs, issueSorts
}

func MoveIssuesOnProjectBoard(ctx context.Context, doer *user_model.User, form *ProjectMovedIssuesForm, project *project_model.Project, board *project_model.Board) error {
	issueIDs, issueSorts := form.ToSortedIssueIDs()

	movedIssues, err := GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		return err
	}

	if len(movedIssues) != len(form.Issues) {
		return errors.New("some issues do not exist")
	}

	if _, err = movedIssues.LoadRepositories(ctx); err != nil {
		return err
	}
	if err = movedIssues.LoadProjects(ctx); err != nil {
		return err
	}
	if err = movedIssues.LoadProjectIssueBoards(ctx); err != nil {
		return err
	}

	for _, issue := range movedIssues {
		if issue.RepoID != project.RepoID && issue.Repo.OwnerID != project.OwnerID {
			return errors.New("Some issue's repoID is not equal to project's repoID")
		}
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		if err = project_model.MoveIssuesOnProjectBoard(ctx, board, issueIDs, issueSorts); err != nil {
			return err
		}

		for _, issue := range movedIssues {
			if issue.ProjectIssue.ProjectBoardID == board.ID {
				continue
			}

			_, err = CreateComment(ctx, &CreateCommentOptions{
				Type:      CommentTypeProjectBoard,
				Doer:      doer,
				Repo:      issue.Repo,
				Issue:     issue,
				ProjectID: project.ID,
				ProjectBoard: &CommentProjectBoardExtendData{
					FromBoardTitle: issue.ProjectIssue.ProjectBoard.Title,
					ToBoardTitle:   board.Title,
				},
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
}
