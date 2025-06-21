// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
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

func findRepoProjectsWorkflows(ctx context.Context, repo *repo_model.Repository) ([]*project_module.Workflow, error) {
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		log.Error("IssueChangeStatus: OpenRepository: %v", err)
		return nil, err
	}
	defer gitRepo.Close()

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(repo.DefaultBranch)
	if err != nil {
		log.Error("gitRepo.GetCommit: %w", err)
		return nil, err
	}

	tree, err := commit.SubTree(".gitea/projects")
	if _, ok := err.(git.ErrNotExist); ok {
		return nil, nil
	}
	if err != nil {
		log.Error("commit.SubTree: %w", err)
		return nil, err
	}

	entries, err := tree.ListEntriesRecursiveFast()
	if err != nil {
		log.Error("tree.ListEntriesRecursiveFast: %w", err)
		return nil, err
	}

	ret := make(git.Entries, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml") {
			ret = append(ret, entry)
		}
	}
	if len(ret) == 0 {
		return nil, nil
	}

	wfs := make([]*project_module.Workflow, 0, len(ret))
	for _, entry := range ret {
		workflowContent, err := commit.GetFileContent(".gitea/projects/"+entry.Name(), 1024*1024)
		if err != nil {
			log.Error("gitRepo.GetCommit: %w", err)
			return nil, err
		}

		wf, err := project_module.ParseWorkflow(workflowContent)
		if err != nil {
			log.Error("IssueChangeStatus: OpenRepository: %v", err)
			return nil, err
		}
		projectName := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".yml"), ".yaml")
		project, err := project_model.GetProjectByName(ctx, repo.ID, projectName)
		if err != nil {
			log.Error("IssueChangeStatus: GetProjectByName: %v", err)
			return nil, err
		}
		wf.ProjectID = project.ID

		wfs = append(wfs, wf)
	}
	return wfs, nil
}

func (m *workflowNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("NewIssue: LoadRepo: %v", err)
		return
	}
	wfs, err := findRepoProjectsWorkflows(ctx, issue.Repo)
	if err != nil {
		log.Error("NewIssue: findRepoProjectsWorkflows: %v", err)
		return
	}

	for _, wf := range wfs {
		if err := wf.FireAction(project_module.EventItemClosed, func(action project_module.Action) error {
			board, err := project_model.GetColumnByProjectIDAndColumnName(ctx, wf.ProjectID, action.SetValue)
			if err != nil {
				log.Error("NewIssue: GetBoardByProjectIDAndBoardName: %v", err)
				return err
			}
			return project_model.AddIssueToColumn(ctx, issue.ID, board)
		}); err != nil {
			log.Error("NewIssue: FireAction: %v", err)
			return
		}
	}
}

func (m *workflowNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	if isClosed {
		if err := issue.LoadRepo(ctx); err != nil {
			log.Error("IssueChangeStatus: LoadRepo: %v", err)
			return
		}
		wfs, err := findRepoProjectsWorkflows(ctx, issue.Repo)
		if err != nil {
			log.Error("IssueChangeStatus: findRepoProjectsWorkflows: %v", err)
			return
		}

		for _, wf := range wfs {
			if err := wf.FireAction(project_module.EventItemClosed, func(action project_module.Action) error {
				board, err := project_model.GetColumnByProjectIDAndColumnName(ctx, wf.ProjectID, action.SetValue)
				if err != nil {
					log.Error("IssueChangeStatus: GetBoardByProjectIDAndBoardName: %v", err)
					return err
				}
				return project_model.MoveIssueToAnotherColumn(ctx, issue.ID, board)
			}); err != nil {
				log.Error("IssueChangeStatus: FireAction: %v", err)
				return
			}
		}
	}
}
