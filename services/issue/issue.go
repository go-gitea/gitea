// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	notify_service "code.gitea.io/gitea/services/notify"
)

// NewIssue creates new issue with labels for repository.
func NewIssue(ctx context.Context, repo *repo_model.Repository, issue *issues_model.Issue, labelIDs []int64, uuids []string, assigneeIDs []int64, projectID int64) error {
	if err := issue.LoadPoster(ctx); err != nil {
		return err
	}

	if user_model.IsUserBlockedBy(ctx, issue.Poster, repo.OwnerID) || user_model.IsUserBlockedBy(ctx, issue.Poster, assigneeIDs...) {
		return user_model.ErrBlockedUser
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := issues_model.NewIssue(ctx, repo, issue, labelIDs, uuids); err != nil {
			return err
		}
		for _, assigneeID := range assigneeIDs {
			if _, err := AddAssigneeIfNotAssigned(ctx, issue, issue.Poster, assigneeID, true); err != nil {
				return err
			}
		}
		if projectID > 0 {
			if err := issues_model.IssueAssignOrRemoveProject(ctx, issue, issue.Poster, projectID, 0); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, issue.Poster, issue.Content)
	if err != nil {
		return err
	}

	notify_service.NewIssue(ctx, issue, mentions)
	if len(issue.Labels) > 0 {
		notify_service.IssueChangeLabels(ctx, issue.Poster, issue, issue.Labels, nil)
	}
	if issue.Milestone != nil {
		notify_service.IssueChangeMilestone(ctx, issue.Poster, issue, 0)
	}

	return nil
}

// ChangeTitle changes the title of this issue, as the given user.
func ChangeTitle(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, title string) error {
	oldTitle := issue.Title
	issue.Title = title

	if oldTitle == title {
		return nil
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	if user_model.IsUserBlockedBy(ctx, doer, issue.PosterID, issue.Repo.OwnerID) {
		if isAdmin, _ := access_model.IsUserRepoAdmin(ctx, issue.Repo, doer); !isAdmin {
			return user_model.ErrBlockedUser
		}
	}

	if err := issues_model.ChangeIssueTitle(ctx, issue, doer, oldTitle); err != nil {
		return err
	}

	var reviewNotifiers []*ReviewRequestNotifier
	if issue.IsPull && issues_model.HasWorkInProgressPrefix(oldTitle) && !issues_model.HasWorkInProgressPrefix(title) {
		var err error
		reviewNotifiers, err = PullRequestCodeOwnersReview(ctx, issue, issue.PullRequest)
		if err != nil {
			log.Error("PullRequestCodeOwnersReview: %v", err)
		}
	}

	notify_service.IssueChangeTitle(ctx, doer, issue, oldTitle)
	ReviewRequestNotify(ctx, issue, issue.Poster, reviewNotifiers)

	return nil
}

// ChangeIssueRef changes the branch of this issue, as the given user.
func ChangeIssueRef(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, ref string) error {
	oldRef := issue.Ref
	issue.Ref = ref

	if err := issues_model.ChangeIssueRef(ctx, issue, doer, oldRef); err != nil {
		return err
	}

	notify_service.IssueChangeRef(ctx, doer, issue, oldRef)

	return nil
}

// UpdateAssignees is a helper function to add or delete one or multiple issue assignee(s)
// Deleting is done the GitHub way (quote from their api documentation):
// https://developer.github.com/v3/issues/#edit-an-issue
// "assignees" (array): Logins for Users to assign to this issue.
// Pass one or more user logins to replace the set of assignees on this Issue.
// Send an empty array ([]) to clear all assignees from the Issue.
func UpdateAssignees(ctx context.Context, issue *issues_model.Issue, oneAssignee string, multipleAssignees []string, doer *user_model.User) (err error) {
	uniqueAssignees := container.SetOf(multipleAssignees...)

	// Keep the old assignee thingy for compatibility reasons
	if oneAssignee != "" {
		uniqueAssignees.Add(oneAssignee)
	}

	// Loop through all assignees to add them
	allNewAssignees := make([]*user_model.User, 0, len(uniqueAssignees))
	for _, assigneeName := range uniqueAssignees.Values() {
		assignee, err := user_model.GetUserByName(ctx, assigneeName)
		if err != nil {
			return err
		}

		if user_model.IsUserBlockedBy(ctx, doer, assignee.ID) {
			return user_model.ErrBlockedUser
		}

		allNewAssignees = append(allNewAssignees, assignee)
	}

	// Delete all old assignees not passed
	if err = DeleteNotPassedAssignee(ctx, issue, doer, allNewAssignees); err != nil {
		return err
	}

	// Add all new assignees
	// Update the assignee. The function will check if the user exists, is already
	// assigned (which he shouldn't as we deleted all assignees before) and
	// has access to the repo.
	for _, assignee := range allNewAssignees {
		// Extra method to prevent double adding (which would result in removing)
		_, err = AddAssigneeIfNotAssigned(ctx, issue, doer, assignee.ID, true)
		if err != nil {
			return err
		}
	}

	return err
}

// DeleteIssue deletes an issue
func DeleteIssue(ctx context.Context, doer *user_model.User, gitRepo *git.Repository, issue *issues_model.Issue) error {
	// load issue before deleting it
	if err := issue.LoadAttributes(ctx); err != nil {
		return err
	}
	if err := issue.LoadPullRequest(ctx); err != nil {
		return err
	}

	// delete entries in database
	if err := deleteIssue(ctx, issue); err != nil {
		return err
	}

	// delete pull request related git data
	if issue.IsPull && gitRepo != nil {
		if err := gitRepo.RemoveReference(fmt.Sprintf("%s%d/head", git.PullPrefix, issue.PullRequest.Index)); err != nil {
			return err
		}
	}

	// If the Issue is pinned, we should unpin it before deletion to avoid problems with other pinned Issues
	if issue.IsPinned() {
		if err := issue.Unpin(ctx, doer); err != nil {
			return err
		}
	}

	notify_service.DeleteIssue(ctx, doer, issue)

	return nil
}

// AddAssigneeIfNotAssigned adds an assignee only if he isn't already assigned to the issue.
// Also checks for access of assigned user
func AddAssigneeIfNotAssigned(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assigneeID int64, notify bool) (comment *issues_model.Comment, err error) {
	assignee, err := user_model.GetUserByID(ctx, assigneeID)
	if err != nil {
		return nil, err
	}

	// Check if the user is already assigned
	isAssigned, err := issues_model.IsUserAssignedToIssue(ctx, issue, assignee)
	if err != nil {
		return nil, err
	}
	if isAssigned {
		// nothing to to
		return nil, nil
	}

	valid, err := access_model.CanBeAssigned(ctx, assignee, issue.Repo, issue.IsPull)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: assigneeID, RepoName: issue.Repo.Name}
	}

	if notify {
		_, comment, err = ToggleAssigneeWithNotify(ctx, issue, doer, assigneeID)
		return comment, err
	}
	_, comment, err = issues_model.ToggleIssueAssignee(ctx, issue, doer, assigneeID)
	return comment, err
}

// GetRefEndNamesAndURLs retrieves the ref end names (e.g. refs/heads/branch-name -> branch-name)
// and their respective URLs.
func GetRefEndNamesAndURLs(issues []*issues_model.Issue, repoLink string) (map[int64]string, map[int64]string) {
	issueRefEndNames := make(map[int64]string, len(issues))
	issueRefURLs := make(map[int64]string, len(issues))
	for _, issue := range issues {
		if issue.Ref != "" {
			issueRefEndNames[issue.ID] = git.RefName(issue.Ref).ShortName()
			issueRefURLs[issue.ID] = git.RefURL(repoLink, issue.Ref)
		}
	}
	return issueRefEndNames, issueRefURLs
}

// deleteIssue deletes the issue
func deleteIssue(ctx context.Context, issue *issues_model.Issue) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	e := db.GetEngine(ctx)
	if _, err := e.ID(issue.ID).NoAutoCondition().Delete(issue); err != nil {
		return err
	}

	// update the total issue numbers
	if err := repo_model.UpdateRepoIssueNumbers(ctx, issue.RepoID, issue.IsPull, false); err != nil {
		return err
	}
	// if the issue is closed, update the closed issue numbers
	if issue.IsClosed {
		if err := repo_model.UpdateRepoIssueNumbers(ctx, issue.RepoID, issue.IsPull, true); err != nil {
			return err
		}
	}

	if err := issues_model.UpdateMilestoneCounters(ctx, issue.MilestoneID); err != nil {
		return fmt.Errorf("error updating counters for milestone id %d: %w",
			issue.MilestoneID, err)
	}

	if err := activities_model.DeleteIssueActions(ctx, issue.RepoID, issue.ID, issue.Index); err != nil {
		return err
	}

	// find attachments related to this issue and remove them
	if err := issue.LoadAttributes(ctx); err != nil {
		return err
	}

	for i := range issue.Attachments {
		system_model.RemoveStorageWithNotice(ctx, storage.Attachments, "Delete issue attachment", issue.Attachments[i].RelativePath())
	}

	// delete all database data still assigned to this issue
	if err := db.DeleteBeans(ctx,
		&issues_model.ContentHistory{IssueID: issue.ID},
		&issues_model.Comment{IssueID: issue.ID},
		&issues_model.IssueLabel{IssueID: issue.ID},
		&issues_model.IssueDependency{IssueID: issue.ID},
		&issues_model.IssueAssignees{IssueID: issue.ID},
		&issues_model.IssueUser{IssueID: issue.ID},
		&activities_model.Notification{IssueID: issue.ID},
		&issues_model.Reaction{IssueID: issue.ID},
		&issues_model.IssueWatch{IssueID: issue.ID},
		&issues_model.Stopwatch{IssueID: issue.ID},
		&issues_model.TrackedTime{IssueID: issue.ID},
		&project_model.ProjectIssue{IssueID: issue.ID},
		&repo_model.Attachment{IssueID: issue.ID},
		&issues_model.PullRequest{IssueID: issue.ID},
		&issues_model.Comment{RefIssueID: issue.ID},
		&issues_model.IssueDependency{DependencyID: issue.ID},
		&issues_model.Comment{DependentIssueID: issue.ID},
	); err != nil {
		return err
	}

	return committer.Commit()
}
