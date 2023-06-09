// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
)

// LoadProject load the project the issue was assigned to
func (issue *Issue) LoadProject(ctx context.Context) (err error) {
	if issue.Projects == nil {
		err = db.GetEngine(ctx).Table("project").
			Join("INNER", "project_issue", "project.id=project_issue.project_id").
			Where("project_issue.issue_id = ?", issue.ID).OrderBy("title").
			Find(&issue.Projects)
	}
	return err
}


func (issue *Issue) projectIDs(ctx context.Context) []int64 {
	var ips []int64
	if err := db.GetEngine(ctx).Table("project_issue").Select("project_id").
		Where("issue_id=?", issue.ID).Find(&ips); err != nil {
		return nil
	}

	return ips
}

// ProjectBoardID return project board id if issue was assigned to one
func (issue *Issue) ProjectBoardID(ctx context.Context) int64 {
	var ip project_model.ProjectIssue
	has, err := db.GetEngine(ctx).Where("issue_id=?", issue.ID).Get(&ip)
	if err != nil || !has {
		return 0
	}
	return ip.ProjectBoardID
}

// LoadIssuesFromBoard load issues assigned to this board
func LoadIssuesFromBoard(ctx context.Context, b *project_model.Board) (IssueList, error) {
	issueList := make(IssueList, 0, 10)

	if b.ID > 0 {
		issues, err := Issues(ctx, &IssuesOptions{
			ProjectBoardID: b.ID,
			ProjectID:      b.ProjectID,
			SortType:       "project-column-sorting",
		})
		if err != nil {
			return nil, err
		}
		issueList = issues
	}

	if b.Default {
		issues, err := Issues(ctx, &IssuesOptions{
			ProjectBoardID: db.NoConditionID,
			ProjectID:      b.ProjectID,
			SortType:       "project-column-sorting",
		})
		if err != nil {
			return nil, err
		}
		issueList = append(issueList, issues...)
	}

	if err := issueList.LoadComments(ctx); err != nil {
		return nil, err
	}

	return issueList, nil
}

// LoadIssuesFromBoardList load issues assigned to the boards
func LoadIssuesFromBoardList(ctx context.Context, bs project_model.BoardList) (map[int64]IssueList, error) {
	issuesMap := make(map[int64]IssueList, len(bs))
	for i := range bs {
		il, err := LoadIssuesFromBoard(ctx, bs[i])
		if err != nil {
			return nil, err
		}
		issuesMap[bs[i].ID] = il
	}
	return issuesMap, nil
}

// ChangeProjectAssign changes the project associated with an issue
func ChangeProjectAssign(ctx context.Context, issue *Issue, doer *user_model.User, newProjectID int64, action string) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := addUpdateIssueProject(ctx, issue, doer, newProjectID, action); err != nil {
		return err
	}

	return committer.Commit()
}

func addUpdateIssueProject(ctx context.Context, issue *Issue, doer *user_model.User, newProjectID int64, action string) error {
	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	oldProjectIDs := issue.projectIDs(ctx)

	if len(oldProjectIDs) > 0 {
		for _, i := range oldProjectIDs {
			// Only check if we add a new project and not remove it.
			if newProjectID > 0 {
				newProject, err := project_model.GetProjectByID(ctx, newProjectID)
				if err != nil {
					return err
				}
				if newProject.RepoID != issue.RepoID && newProject.OwnerID != issue.Repo.OwnerID {
					return fmt.Errorf("issue's repository is not the same as project's repository")
				}
			}

			if action == "attach" && newProjectID > 0 {
				if err := db.Insert(ctx, &project_model.ProjectIssue{
					IssueID:   issue.ID,
					ProjectID: newProjectID,
				}); err != nil {
					return err
				}
				i = 0
			} else {
				if action == "clear" {
					if _, err := db.GetEngine(ctx).Where("project_issue.issue_id=?", issue.ID).Delete(&project_model.ProjectIssue{}); err != nil {
						return err
					}
				} else {
					i = newProjectID
					newProjectID = 0
					if _, err := db.GetEngine(ctx).Where("project_issue.issue_id=? AND project_issue.project_id=?", issue.ID, i).Delete(&project_model.ProjectIssue{}); err != nil {
						return err
					}
				}
			}

			if i > 0 || newProjectID > 0 {
				if _, err := CreateComment(ctx, &CreateCommentOptions{
					Type:         CommentTypeProject,
					Doer:         doer,
					Repo:         issue.Repo,
					Issue:        issue,
					OldProjectID: i,
					ProjectID:    newProjectID,
				}); err != nil {
					return err
				}
			}
			if action != "clear" && newProjectID == 0 || newProjectID > 0 {
				break
			}
		}
	} else {
		if action == "attach" || action == "" {
			if err := db.Insert(ctx, &project_model.ProjectIssue{
				IssueID:   issue.ID,
				ProjectID: newProjectID,
			}); err != nil {
				return err
			}
		}

		if newProjectID > 0 {
			if _, err := CreateComment(ctx, &CreateCommentOptions{
				Type:         CommentTypeProject,
				Doer:         doer,
				Repo:         issue.Repo,
				Issue:        issue,
				OldProjectID: 0,
				ProjectID:    newProjectID,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
