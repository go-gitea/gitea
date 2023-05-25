// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	organization_model "code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/storage"
)

// NewIssue creates new issue with labels for repository.
func NewIssue(ctx context.Context, repo *repo_model.Repository, issue *issues_model.Issue, labelIDs []int64, uuids []string, assigneeIDs []int64) error {
	if err := issues_model.NewIssue(repo, issue, labelIDs, uuids); err != nil {
		return err
	}

	for _, assigneeID := range assigneeIDs {
		if err := AddAssigneeIfNotAssigned(ctx, issue, issue.Poster, assigneeID); err != nil {
			return err
		}
	}

	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, issue.Poster, issue.Content)
	if err != nil {
		return err
	}

	notification.NotifyNewIssue(ctx, issue, mentions)
	if len(issue.Labels) > 0 {
		notification.NotifyIssueChangeLabels(ctx, issue.Poster, issue, issue.Labels, nil)
	}
	if issue.Milestone != nil {
		notification.NotifyIssueChangeMilestone(ctx, issue.Poster, issue, 0)
	}

	return nil
}

// ChangeTitle changes the title of this issue, as the given user.
func ChangeTitle(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, title string) (err error) {
	oldTitle := issue.Title
	issue.Title = title

	if err = issues_model.ChangeIssueTitle(ctx, issue, doer, oldTitle); err != nil {
		return
	}

	notification.NotifyIssueChangeTitle(ctx, doer, issue, oldTitle)

	return nil
}

// ChangeIssueRef changes the branch of this issue, as the given user.
func ChangeIssueRef(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, ref string) error {
	oldRef := issue.Ref
	issue.Ref = ref

	if err := issues_model.ChangeIssueRef(issue, doer, oldRef); err != nil {
		return err
	}

	notification.NotifyIssueChangeRef(ctx, doer, issue, oldRef)

	return nil
}

// UpdateAssignees is a helper function to add or delete one or multiple issue assignee(s)
// Deleting is done the GitHub way (quote from their api documentation):
// https://developer.github.com/v3/issues/#edit-an-issue
// "assignees" (array): Logins for Users to assign to this issue.
// Pass one or more user logins to replace the set of assignees on this Issue.
// Send an empty array ([]) to clear all assignees from the Issue.
func UpdateAssignees(ctx context.Context, issue *issues_model.Issue, oneAssignee string, multipleAssignees []string, doer *user_model.User) (err error) {
	var allNewAssignees []*user_model.User

	// Keep the old assignee thingy for compatibility reasons
	if oneAssignee != "" {
		// Prevent double adding assignees
		var isDouble bool
		for _, assignee := range multipleAssignees {
			if assignee == oneAssignee {
				isDouble = true
				break
			}
		}

		if !isDouble {
			multipleAssignees = append(multipleAssignees, oneAssignee)
		}
	}

	// Loop through all assignees to add them
	for _, assigneeName := range multipleAssignees {
		assignee, err := user_model.GetUserByName(ctx, assigneeName)
		if err != nil {
			return err
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
		err = AddAssigneeIfNotAssigned(ctx, issue, doer, assignee.ID)
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

	notification.NotifyDeleteIssue(ctx, doer, issue)

	return nil
}

// AddAssigneeIfNotAssigned adds an assignee only if he isn't already assigned to the issue.
// Also checks for access of assigned user
func AddAssigneeIfNotAssigned(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assigneeID int64) (err error) {
	assignee, err := user_model.GetUserByID(ctx, assigneeID)
	if err != nil {
		return err
	}

	// Check if the user is already assigned
	isAssigned, err := issues_model.IsUserAssignedToIssue(ctx, issue, assignee)
	if err != nil {
		return err
	}
	if isAssigned {
		// nothing to to
		return nil
	}

	valid, err := access_model.CanBeAssigned(ctx, assignee, issue.Repo, issue.IsPull)
	if err != nil {
		return err
	}
	if !valid {
		return repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: assigneeID, RepoName: issue.Repo.Name}
	}

	_, _, err = ToggleAssignee(ctx, issue, doer, assigneeID)
	if err != nil {
		return err
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// AddCodeownerReviewers gets all the codeowners of the files changed in the pull request (as outlined in the base repository's
// CODEOWNERS file) and requests them for review if they exist and are eligibl to do so. Codeowners can be users or teams.
func AddCodeownerReviewers(ctx context.Context, pr *issues_model.PullRequest, repo *repo_model.Repository) (err error) {
	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	defer gitRepo.Close()
	if err != nil {
		log.Error("git.OpenRepository: %v", err)
		return err
	}

	codeownersContents, err := GetCodeownersFileContents(ctx, pr, gitRepo)
	if err != nil {
		return err
	}

	if codeownersContents != nil {
		changedFiles, err := gitRepo.GetFilesChangedBetween(pr.MergeBase, pr.HeadCommitID)
		if err != nil {
			log.Error("git.Repository.GetFilesChangedBetween: %v", err)
			return err
		}

		owners, teamOwners, err := ParseCodeowners(changedFiles, codeownersContents)
		if err != nil {
			log.Error("ParseCodeowners: %v", err)
			return nil
		}

		err = AddValidReviewers(ctx, pr.Issue, repo, owners, teamOwners)
		if err != nil {
			log.Error("AddValidReviewers: %v", err)
			return err
		}
	}
	return nil
}

// AddValidReviewers adds reviewers to the given issue (pull request) if the users and teams are valid and eligible to do so
func AddValidReviewers(ctx context.Context, issue *issues_model.Issue, repo *repo_model.Repository, owners []string, teamOwners []string) error {
	prPoster := issue.Poster
	isAdd := true

	permDoer, err := access_model.GetUserRepoPermission(ctx, repo, prPoster)
	if err != nil {
		return err
	}

	// Errors here should not cause the process to fail.
	for _, userReviewer := range GetValidUserReviewers(ctx, owners, prPoster, isAdd, issue, &permDoer) {
		_, err = ReviewRequest(ctx, issue, prPoster, userReviewer, isAdd)
		if err != nil {
			log.Warn("AddValidReviewers [repo_id: %d, issue_id: %d, pull_request_poster_user_id: %d, user_reviewer_id: %d]: "+
				"Error adding user as a reviewer to the pull request", repo.ID, issue.ID, prPoster.ID, userReviewer.ID)
		}
	}
	for _, teamReviewer := range GetValidTeamReviewers(ctx, repo, teamOwners, prPoster, isAdd, issue) {
		_, err = TeamReviewRequest(ctx, issue, prPoster, teamReviewer, isAdd)
		if err != nil {
			log.Warn("AddValidReviewers [repo_id: %d, issue_id: %d, pull_request_poster_user_id: %d, team_reviewer_id: %d]: "+
				"Error adding team as a reviewer to the pull request", repo.ID, issue.ID, prPoster.ID, teamReviewer.ID)
		}
	}

	return nil
}

// FindCodeownersFile gets the CODEOWNERS file from the top level,'.gitea', or 'docs' directory of the given repository.
func GetCodeownersFileContents(ctx context.Context, pr *issues_model.PullRequest, gitRepo *git.Repository) ([]byte, error) {
	// TODO: include these directories as constants in codeowners.go after refactor
	possibleDirectories := []string{"", ".gitea/", "docs/"} // accepted directories to search for the CODEOWNERS file

	commit, err := gitRepo.GetCommit(pr.BaseBranch)
	if err != nil {
		return nil, err
	}

	entry := GetCodeownersTreeEntry(commit, possibleDirectories)
	if entry == nil {
		return nil, nil
	}

	if entry.IsRegular() {
		gitBlob := entry.Blob()
		data, err := gitBlob.GetBlobContentBase64()
		if err != nil {
			return nil, err
		}
		contentBytes, err := b64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, err
		}
		// TODO: Include as constant in codeowners.go after refactor
		byteLimit := 3 * 1024 * 1024 // 3 MB limit, per GitHub specs,
		if len(contentBytes) >= byteLimit {
			log.Info("GetCodeownersFileContents [repo_id: %d, pr_id: %d, git_tree_entry_id: %d, content_num_bytes: %d, byte_limit: %d]: "+
				"CODEOWNERS file exceeds accepted size limit", pr.Issue.RepoID, pr.ID, entry.ID, len(contentBytes), byteLimit)
			return nil, nil
		}
		return contentBytes, nil
	}
	log.Warn("GetCodeownersFileContents [repo_id: %d, git_tree_entry_id: %d]: CODEOWNERS file found is not a regular file", pr.Issue.RepoID, entry.ID)

	return nil, nil
}

// GetCodeownersTreeEntry gets the git tree entry of the CODEOWNERS file, given an array of directories to search in. Nil if not found.
func GetCodeownersTreeEntry(commit *git.Commit, directoryOptions []string) *git.TreeEntry {
	for _, dir := range directoryOptions {
		entry, _ := commit.GetTreeEntryByPath(dir + "CODEOWNERS")
		if entry != nil {
			return entry
		}
	}
	return nil
}

// GetValidReviewers gets the Users that actually exist and are authorized to review the pull request
func GetValidUserReviewers(ctx context.Context, userNamesOrEmails []string, doer *user_model.User, isAdd bool, issue *issues_model.Issue, permDoer *access_model.Permission) (reviewers []*user_model.User) {
	reviewers = []*user_model.User{}
	for _, nameOrEmail := range userNamesOrEmails {
		var reviewer *user_model.User
		var err error
		if strings.Contains(nameOrEmail, "@") {
			reviewer, err = user_model.GetUserByEmail(ctx, nameOrEmail)
			if err != nil {
				log.Info("GetValidUserReviewers [repo_id: %d, owner_email: %s]: user owner in CODEOWNERS file could not be found by email", issue.RepoID, nameOrEmail)
			}
		} else {
			reviewer, err = user_model.GetUserByName(ctx, nameOrEmail)
			if err != nil {
				log.Info("GetValidUserReviewers [repo_id: %d, owner_username: %s]: user owner in CODEOWNERS file could not be found by name", issue.RepoID, nameOrEmail)
			}
		}
		if reviewer != nil && err == nil {
			err = IsValidReviewRequest(ctx, reviewer, doer, isAdd, issue, permDoer)
			if err == nil {
				reviewers = append(reviewers, reviewer)
			} else {
				log.Info("GetValidUserReviewers [repo_id: %d, user_id: %d]: user reviewer is not a valid review request", issue.RepoID, reviewer.ID)
			}
		}
	}
	return reviewers
}

// GetValidReviewers gets the Teams that actually exist and are authorized to review the pull request
func GetValidTeamReviewers(ctx context.Context, repo *repo_model.Repository, fullTeamNames []string, doer *user_model.User, isAdd bool, issue *issues_model.Issue) (teamReviewers []*organization_model.Team) {
	teamReviewers = []*organization_model.Team{}
	if repo.Owner.IsOrganization() {
		for _, fullTeamName := range fullTeamNames {
			teamReviewer, err := GetTeamFromFullName(ctx, fullTeamName, doer)
			if err != nil {
				log.Info("GetTeamFromFullName [repo_id: %d, full_team_name: %s]: error finding the team [%v]", repo.ID, fullTeamName, err)
			} else if teamReviewer == nil {
				log.Info("GetTeamFromFullName [repo_id: %d, full_team_name: %s]: no error returned, but the team was nil (could not be found)", repo.ID, fullTeamName)
			} else {
				err := IsValidTeamReviewRequest(ctx, teamReviewer, doer, isAdd, issue)
				if err == nil {
					teamReviewers = append(teamReviewers, teamReviewer)
				}
				log.Info("GetValidTeamReviewers [repo_id: %d, team_id: %d]: team reviewer is not a valid review request", repo.ID, teamReviewer.ID)
			}
		}
	}
	return teamReviewers
}

// GetTeamFromFullName gets the team given its full name ('{organizationName}/{teamName}')
func GetTeamFromFullName(ctx context.Context, fullTeamName string, doer *user_model.User) (*organization_model.Team, error) {
	teamNameSplit := strings.Split(fullTeamName, "/")
	if len(teamNameSplit) != 2 {
		return nil, errors.New("Team name must split into exactly 2 parts when split on '/'")
	}
	organizationName, teamName := teamNameSplit[0], teamNameSplit[1]

	opts := organization_model.FindOrgOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		UserID:         doer.ID,
		IncludePrivate: true,
	}
	organizations, err := organization_model.FindOrgs(opts)
	if err != nil {
		return nil, err
	}

	var organization *organization_model.Organization
	for _, org := range organizations {
		if org.Name == organizationName {
			organization = org
			break
		}
	}

	var team *organization_model.Team
	if organization != nil {
		team, err = organization.GetTeam(ctx, teamName)
		if err != nil {
			return nil, err
		}
	}
	return team, nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetRefEndNamesAndURLs retrieves the ref end names (e.g. refs/heads/branch-name -> branch-name)
// and their respective URLs.
func GetRefEndNamesAndURLs(issues []*issues_model.Issue, repoLink string) (map[int64]string, map[int64]string) {
	issueRefEndNames := make(map[int64]string, len(issues))
	issueRefURLs := make(map[int64]string, len(issues))
	for _, issue := range issues {
		if issue.Ref != "" {
			issueRefEndNames[issue.ID] = git.RefEndName(issue.Ref)
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

	if err := activities_model.DeleteIssueActions(ctx, issue.RepoID, issue.ID); err != nil {
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
	if err := issues_model.DeleteInIssue(ctx, issue.ID,
		&issues_model.ContentHistory{},
		&issues_model.Comment{},
		&issues_model.IssueLabel{},
		&issues_model.IssueDependency{},
		&issues_model.IssueAssignees{},
		&issues_model.IssueUser{},
		&activities_model.Notification{},
		&issues_model.Reaction{},
		&issues_model.IssueWatch{},
		&issues_model.Stopwatch{},
		&issues_model.TrackedTime{},
		&project_model.ProjectIssue{},
		&repo_model.Attachment{},
		&issues_model.PullRequest{},
	); err != nil {
		return err
	}

	// References to this issue in other issues
	if _, err := db.DeleteByBean(ctx, &issues_model.Comment{
		RefIssueID: issue.ID,
	}); err != nil {
		return err
	}

	// Delete dependencies for issues in other repositories
	if _, err := db.DeleteByBean(ctx, &issues_model.IssueDependency{
		DependencyID: issue.ID,
	}); err != nil {
		return err
	}

	// delete from dependent issues
	if _, err := db.DeleteByBean(ctx, &issues_model.Comment{
		DependentIssueID: issue.ID,
	}); err != nil {
		return err
	}

	return committer.Commit()
}
