// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	project_module "code.gitea.io/gitea/modules/projects"
	notify_service "code.gitea.io/gitea/services/notify"
)

func init() {
	notify_service.RegisterNotifier(&workflowNotifier{})
}

type workflowNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &workflowNotifier{}

// NewNotifier create a new workflowNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &workflowNotifier{}
}

func (m *workflowNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	if isClosed {
		if err := issue.LoadRepo(ctx); err != nil {
			log.Error("IssueChangeStatus: LoadRepo: %v", err)
			return
		}
		gitRepo, err := gitrepo.OpenRepository(ctx, issue.Repo)
		if err != nil {
			log.Error("IssueChangeStatus: OpenRepository: %v", err)
			return
		}
		defer gitRepo.Close()

		// Get the commit object for the ref
		commit, err := gitRepo.GetCommit(issue.Repo.DefaultBranch)
		if err != nil {
			log.Error("gitRepo.GetCommit: %w", err)
			return
		}

		tree, err := commit.SubTree(".gitea/projects")
		if _, ok := err.(git.ErrNotExist); ok {
			return
		}
		if err != nil {
			log.Error("commit.SubTree: %w", err)
			return
		}

		entries, err := tree.ListEntriesRecursiveFast()
		if err != nil {
			log.Error("tree.ListEntriesRecursiveFast: %w", err)
			return
		}

		ret := make(git.Entries, 0, len(entries))
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml") {
				ret = append(ret, entry)
			}
		}
		if len(ret) == 0 {
			return
		}

		for _, entry := range ret {
			workflowContent, err := commit.GetFileContent(".gitea/projects/"+entry.Name(), 1024*1024)
			if err != nil {
				log.Error("gitRepo.GetCommit: %w", err)
				return
			}

			wf, err := project_module.ParseWorkflow(workflowContent)
			if err != nil {
				log.Error("IssueChangeStatus: OpenRepository: %v", err)
				return
			}
			projectName := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".yml"), ".yaml")
			project, err := project_model.GetProjectByName(ctx, issue.RepoID, projectName)
			if err != nil {
				log.Error("IssueChangeStatus: GetProjectByName: %v", err)
				return
			}
			if err := wf.FireAction(project_module.EventItemClosed, func(action project_module.Action) error {
				board, err := project_model.GetBoardByProjectIDAndBoardName(ctx, project.ID, action.SetValue)
				if err != nil {
					log.Error("IssueChangeStatus: GetBoardByProjectIDAndBoardName: %v", err)
					return err
				}
				return project_model.MoveIssueToAnotherBoard(ctx, issue.ID, board)
			}); err != nil {
				log.Error("IssueChangeStatus: FireAction: %v", err)
				return
			}
		}
	}
}
