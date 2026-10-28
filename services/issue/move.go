// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	issue_indexer "gitea.dev/modules/indexer/issues"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/util"

	"github.com/google/uuid"
)

// MoveIssueOptions are the options for MoveIssue
type MoveIssueOptions struct {
	// KeepLabels finds or creates labels with the same names in the target repository
	KeepLabels bool
	// KeepMilestone finds or creates a milestone with the same name in the target repository
	KeepMilestone bool
	// KeepProjects finds or creates projects with the same titles in the target repository
	KeepProjects bool
}

// CopyIssueDependency is the dependency to create between the original issue and its copy
type CopyIssueDependency string

const (
	CopyIssueDependencyNone               CopyIssueDependency = ""
	CopyIssueDependencyOriginalBlocksCopy CopyIssueDependency = "original_blocks_copy"
	CopyIssueDependencyCopyBlocksOriginal CopyIssueDependency = "copy_blocks_original"
)

// CopyIssueOptions are the options for CopyIssue
type CopyIssueOptions struct {
	MoveIssueOptions
	// CopyAttachments duplicates the attachments of the issue body
	CopyAttachments bool
	// CopyAssignees keeps the assignees who can access the target repository
	CopyAssignees bool
	// Dependency links the original issue and its copy
	Dependency CopyIssueDependency
}

func checkMoveIssueTarget(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, targetRepo *repo_model.Repository) error {
	if issue.IsPull {
		return util.NewInvalidArgumentErrorf("pull requests can not be moved or copied to another repository")
	}
	if issue.RepoID == targetRepo.ID {
		return util.NewInvalidArgumentErrorf("the issue is already in the target repository")
	}
	if targetRepo.IsArchived {
		return util.NewInvalidArgumentErrorf("the target repository is archived")
	}
	perm, err := access_model.GetDoerRepoPermission(ctx, targetRepo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWrite(unit.TypeIssues) {
		return util.NewPermissionDeniedErrorf("no permission to write issues in the target repository")
	}
	return nil
}

// findOrCreateTargetLabels maps the labels to the target repository.
// Org labels of the target owner are kept, others are matched by name or created.
func findOrCreateTargetLabels(ctx context.Context, labels []*issues_model.Label, targetRepo *repo_model.Repository) ([]*issues_model.Label, error) {
	result := make([]*issues_model.Label, 0, len(labels))
	for _, label := range labels {
		if label.BelongsToOrg() && label.OrgID == targetRepo.OwnerID {
			result = append(result, label)
			continue
		}
		targetLabel, err := issues_model.GetLabelInRepoByName(ctx, targetRepo.ID, label.Name)
		if errors.Is(err, util.ErrNotExist) {
			targetLabel = &issues_model.Label{
				RepoID:         targetRepo.ID,
				Name:           label.Name,
				Exclusive:      label.Exclusive,
				ExclusiveOrder: label.ExclusiveOrder,
				Description:    label.Description,
				Color:          label.Color,
			}
			err = issues_model.NewLabel(ctx, targetLabel)
		}
		if err != nil {
			return nil, err
		}
		result = append(result, targetLabel)
	}
	return result, nil
}

// findOrCreateTargetMilestone maps the milestone to the target repository by name.
func findOrCreateTargetMilestone(ctx context.Context, milestone *issues_model.Milestone, targetRepo *repo_model.Repository) (int64, error) {
	targetMilestone, err := issues_model.GetMilestoneByRepoIDANDName(ctx, targetRepo.ID, milestone.Name)
	if issues_model.IsErrMilestoneNotExist(err) {
		targetMilestone = &issues_model.Milestone{
			RepoID:       targetRepo.ID,
			Name:         milestone.Name,
			Content:      milestone.Content,
			DeadlineUnix: milestone.DeadlineUnix,
		}
		err = issues_model.NewMilestone(ctx, targetMilestone)
	}
	if err != nil {
		return 0, err
	}
	return targetMilestone.ID, nil
}

// findOrCreateTargetProjects maps the projects to the target repository.
// Projects which can still be accessed from the target repository are kept, others are matched by title or created.
func findOrCreateTargetProjects(ctx context.Context, doer *user_model.User, projects []*project_model.Project, targetRepo *repo_model.Repository) ([]int64, error) {
	if len(projects) == 0 {
		return nil, nil
	}
	repoProjects, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
		RepoID: targetRepo.ID,
		Type:   project_model.TypeRepository,
	})
	if err != nil {
		return nil, err
	}
	result := make([]int64, 0, len(projects))
	for _, project := range projects {
		if project.CanBeAccessedByOwnerRepo(targetRepo.OwnerID, targetRepo) {
			result = append(result, project.ID)
			continue
		}
		var projectID int64
		for _, p := range repoProjects {
			if p.Title == project.Title {
				projectID = p.ID
				break
			}
		}
		if projectID == 0 {
			newProject := &project_model.Project{
				Title:        project.Title,
				Description:  project.Description,
				RepoID:       targetRepo.ID,
				CreatorID:    doer.ID,
				TemplateType: project.TemplateType,
				CardType:     project.CardType,
				Type:         project_model.TypeRepository,
			}
			if err := project_model.NewProject(ctx, newProject); err != nil {
				return nil, err
			}
			repoProjects = append(repoProjects, newProject)
			projectID = newProject.ID
		}
		result = append(result, projectID)
	}
	return result, nil
}

func canReadIssues(ctx context.Context, user *user_model.User, repo *repo_model.Repository) (bool, error) {
	perm, err := access_model.GetIndividualUserRepoPermission(ctx, repo, user)
	if err != nil {
		return false, err
	}
	return perm.CanRead(unit.TypeIssues), nil
}

// MoveIssue moves an issue to another repository.
// The issue gets a new index in the target repository, and a redirect is recorded for the old URL.
func MoveIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, targetRepo *repo_model.Repository, opts MoveIssueOptions) error {
	if err := checkMoveIssueTarget(ctx, doer, issue, targetRepo); err != nil {
		return err
	}
	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}
	if err := issue.LoadLabels(ctx); err != nil {
		return err
	}
	if err := issue.LoadMilestone(ctx); err != nil {
		return err
	}
	if err := issue.LoadProjects(ctx); err != nil {
		return err
	}
	if err := issue.LoadAssignees(ctx); err != nil {
		return err
	}
	oldRepo := issue.Repo
	oldIndex := issue.Index

	err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := issues_model.UnpinIssue(ctx, issue, doer); err != nil {
			return err
		}

		newIndex, err := db.GetNextResourceIndex(ctx, "issue_index", targetRepo.ID)
		if err != nil {
			return err
		}
		issue.RepoID, issue.Repo, issue.Index = targetRepo.ID, targetRepo, newIndex
		if err := issues_model.UpdateIssueCols(ctx, issue, "repo_id", "index"); err != nil {
			return err
		}
		if err := issues_model.CreateIssueRedirect(ctx, oldRepo.ID, oldIndex, issue.ID); err != nil {
			return err
		}

		if err := issues_model.DecrRepoIssueNumbers(ctx, oldRepo.ID, false, true, issue.IsClosed); err != nil {
			return err
		}
		if err := issues_model.IncrRepoIssueNumbers(ctx, targetRepo.ID, false, true); err != nil {
			return err
		}
		if issue.IsClosed {
			if err := issues_model.IncrRepoIssueNumbers(ctx, targetRepo.ID, false, false); err != nil {
				return err
			}
		}

		// attachments of the issue body and its comments, and the notifications follow the issue
		if _, err := db.GetEngine(ctx).Where("issue_id = ?", issue.ID).Cols("repo_id").
			Update(&repo_model.Attachment{RepoID: targetRepo.ID}); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Exec("UPDATE `notification` SET repo_id = ? WHERE issue_id = ?", targetRepo.ID, issue.ID); err != nil {
			return err
		}
		// cross references made by this issue point to its repository
		if _, err := db.GetEngine(ctx).Exec("UPDATE `comment` SET ref_repo_id = ? WHERE ref_issue_id = ? AND ref_repo_id = ?", targetRepo.ID, issue.ID, oldRepo.ID); err != nil {
			return err
		}

		// labels, milestone and projects belong to the old repository: map or drop them
		var labels []*issues_model.Label
		if opts.KeepLabels {
			if labels, err = findOrCreateTargetLabels(ctx, issue.Labels, targetRepo); err != nil {
				return err
			}
		}
		if err := issues_model.ReplaceIssueLabels(ctx, issue, labels, doer); err != nil {
			return err
		}

		if issue.Milestone != nil {
			oldMilestoneID := issue.MilestoneID
			issue.MilestoneID = 0
			if opts.KeepMilestone {
				if issue.MilestoneID, err = findOrCreateTargetMilestone(ctx, issue.Milestone, targetRepo); err != nil {
					return err
				}
			}
			if err := changeMilestoneAssign(ctx, doer, issue, oldMilestoneID); err != nil {
				return err
			}
		}

		var projectIDs []int64
		for _, project := range issue.Projects {
			if project.CanBeAccessedByOwnerRepo(targetRepo.OwnerID, targetRepo) {
				projectIDs = append(projectIDs, project.ID)
			}
		}
		if opts.KeepProjects {
			if projectIDs, err = findOrCreateTargetProjects(ctx, doer, issue.Projects, targetRepo); err != nil {
				return err
			}
		}
		if _, _, err := issues_model.IssueAssignOrRemoveProject(ctx, issue, doer, projectIDs); err != nil {
			return err
		}

		// users who can't read issues in the target repository can't stay assigned or subscribed
		for _, assignee := range issue.Assignees {
			canRead, err := canReadIssues(ctx, assignee, targetRepo)
			if err != nil {
				return err
			}
			if !canRead {
				if _, _, err := issues_model.ToggleIssueAssignee(ctx, issue, doer, assignee.ID); err != nil {
					return err
				}
			}
		}
		watchers, err := issues_model.GetIssueWatchers(ctx, issue.ID, db.ListOptions{})
		if err != nil {
			return err
		}
		for _, watch := range watchers {
			watcher, err := user_model.GetUserByID(ctx, watch.UserID)
			if err != nil {
				return err
			}
			canRead, err := canReadIssues(ctx, watcher, targetRepo)
			if err != nil {
				return err
			}
			if !canRead {
				if _, err := db.GetEngine(ctx).ID(watch.ID).Delete(new(issues_model.IssueWatch)); err != nil {
					return err
				}
			}
		}

		if !setting.Service.AllowCrossRepositoryDependencies {
			if err := removeCrossRepoDependencies(ctx, doer, issue); err != nil {
				return err
			}
		}

		_, err = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
			Type:    issues_model.CommentTypeMoveIssueFrom,
			Doer:    doer,
			Repo:    targetRepo,
			Issue:   issue,
			Content: fmt.Sprintf("%s#%d", oldRepo.FullName(), oldIndex),
		})
		return err
	})
	if err != nil {
		return err
	}

	issue_indexer.UpdateIssueIndexer(ctx, issue.ID)
	return nil
}

// removeCrossRepoDependencies removes the dependencies between the issue and issues of other repositories
func removeCrossRepoDependencies(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) error {
	blockedBy, _, err := issue.BlockedByDependencies(ctx, db.ListOptions{})
	if err != nil {
		return err
	}
	for _, dep := range blockedBy {
		if dep.Issue.RepoID != issue.RepoID {
			if err := issues_model.RemoveIssueDependency(ctx, doer, issue, &dep.Issue, issues_model.DependencyTypeBlockedBy); err != nil {
				return err
			}
		}
	}
	blocking, err := issue.BlockingDependencies(ctx)
	if err != nil {
		return err
	}
	for _, dep := range blocking {
		if dep.Issue.RepoID != issue.RepoID {
			if err := issues_model.RemoveIssueDependency(ctx, doer, issue, &dep.Issue, issues_model.DependencyTypeBlocking); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyIssueAttachments duplicates the attachments of the issue body for the target repository.
// It returns the new attachment UUIDs and the content with the attachment links replaced.
func copyIssueAttachments(ctx context.Context, issue *issues_model.Issue, targetRepo *repo_model.Repository) (uuids []string, content string, err error) {
	attachments, err := repo_model.GetAttachmentsByIssueID(ctx, issue.ID)
	if err != nil {
		return nil, "", err
	}
	content = issue.Content
	for _, attachment := range attachments {
		if attachment.CommentID != 0 {
			continue
		}
		newAttachment := &repo_model.Attachment{
			UUID:       uuid.New().String(),
			RepoID:     targetRepo.ID,
			UploaderID: attachment.UploaderID,
			Name:       attachment.Name,
			Size:       attachment.Size,
		}
		if _, err := storage.Copy(storage.Attachments, newAttachment.RelativePath(), storage.Attachments, attachment.RelativePath()); err != nil {
			return nil, "", err
		}
		if err := db.Insert(ctx, newAttachment); err != nil {
			return nil, "", err
		}
		uuids = append(uuids, newAttachment.UUID)
		content = strings.ReplaceAll(content, attachment.UUID, newAttachment.UUID)
	}
	return uuids, content, nil
}

// CopyIssue creates a copy of an issue in another repository and returns the new issue.
func CopyIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, targetRepo *repo_model.Repository, opts CopyIssueOptions) (*issues_model.Issue, error) {
	if err := checkMoveIssueTarget(ctx, doer, issue, targetRepo); err != nil {
		return nil, err
	}
	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}
	if opts.Dependency != CopyIssueDependencyNone {
		if opts.Dependency != CopyIssueDependencyOriginalBlocksCopy && opts.Dependency != CopyIssueDependencyCopyBlocksOriginal {
			return nil, util.NewInvalidArgumentErrorf("unknown dependency type %q", opts.Dependency)
		}
		if !setting.Service.AllowCrossRepositoryDependencies {
			return nil, util.NewInvalidArgumentErrorf("cross repository dependencies are not allowed")
		}
		if !issue.Repo.IsDependenciesEnabled(ctx) || !targetRepo.IsDependenciesEnabled(ctx) {
			return nil, util.NewInvalidArgumentErrorf("dependencies are not enabled in both repositories")
		}
	}

	newIssue := &issues_model.Issue{
		RepoID:   targetRepo.ID,
		Repo:     targetRepo,
		PosterID: issue.PosterID,
		Title:    issue.Title,
		Content:  issue.Content,
	}
	var labelIDs, assigneeIDs, projectIDs []int64
	var uuids []string
	err := db.WithTx(ctx, func(ctx context.Context) error {
		if opts.KeepLabels {
			if err := issue.LoadLabels(ctx); err != nil {
				return err
			}
			labels, err := findOrCreateTargetLabels(ctx, issue.Labels, targetRepo)
			if err != nil {
				return err
			}
			for _, label := range labels {
				labelIDs = append(labelIDs, label.ID)
			}
		}
		if opts.KeepMilestone {
			if err := issue.LoadMilestone(ctx); err != nil {
				return err
			}
			if issue.Milestone != nil {
				milestoneID, err := findOrCreateTargetMilestone(ctx, issue.Milestone, targetRepo)
				if err != nil {
					return err
				}
				newIssue.MilestoneID = milestoneID
			}
		}
		if opts.KeepProjects {
			if err := issue.LoadProjects(ctx); err != nil {
				return err
			}
			var err error
			if projectIDs, err = findOrCreateTargetProjects(ctx, doer, issue.Projects, targetRepo); err != nil {
				return err
			}
		}
		if opts.CopyAttachments {
			var err error
			if uuids, newIssue.Content, err = copyIssueAttachments(ctx, issue, targetRepo); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if opts.CopyAssignees {
		if err := issue.LoadAssignees(ctx); err != nil {
			return nil, err
		}
		for _, assignee := range issue.Assignees {
			canRead, err := canReadIssues(ctx, assignee, targetRepo)
			if err != nil {
				return nil, err
			}
			if canRead {
				assigneeIDs = append(assigneeIDs, assignee.ID)
			}
		}
	}

	if err := NewIssue(ctx, targetRepo, newIssue, labelIDs, uuids, assigneeIDs, projectIDs); err != nil {
		return nil, err
	}

	switch opts.Dependency {
	case CopyIssueDependencyOriginalBlocksCopy:
		err = issues_model.CreateIssueDependency(ctx, doer, newIssue, issue)
	case CopyIssueDependencyCopyBlocksOriginal:
		err = issues_model.CreateIssueDependency(ctx, doer, issue, newIssue)
	}
	if err != nil {
		log.Error("CreateIssueDependency between issue %d and its copy %d: %v", issue.ID, newIssue.ID, err)
	}

	if _, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:    issues_model.CommentTypeCopyIssueTo,
		Doer:    doer,
		Repo:    issue.Repo,
		Issue:   issue,
		Content: fmt.Sprintf("%s#%d", targetRepo.FullName(), newIssue.Index),
	}); err != nil {
		return nil, err
	}
	if _, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:    issues_model.CommentTypeCopyIssueFrom,
		Doer:    doer,
		Repo:    targetRepo,
		Issue:   newIssue,
		Content: fmt.Sprintf("%s#%d", issue.Repo.FullName(), issue.Index),
	}); err != nil {
		return nil, err
	}
	return newIssue, nil
}
