// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"

	// "io/ioutil"
	// "net/http"
	b64 "encoding/base64"
	// "os"
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
	if err != nil {
		// TODO: Where to log?
		return err
	}
	defer gitRepo.Close()

	codeownersContents := GetCodeownersFileContents(ctx, pr, gitRepo)
	if codeownersContents != nil {
		changedFiles, err := gitRepo.GetFilesChangedBetween(pr.MergeBase, pr.HeadCommitID)
		if err != nil {
			// TODO: Where to log?
			return err
		}

		owners, teamOwners, err := ParseCodeowners(changedFiles, codeownersContents)
		if err != nil {
			// TODO: Log the parsing error?
			return nil
		}

		AddValidReviewers(ctx, pr.Issue, repo, owners, teamOwners)
	}
	return nil
}

// AddValidReviewers adds reviewers to the given issue (pull request) if the users and teams are valid and eligible to do so
func AddValidReviewers(ctx context.Context, issue *issues_model.Issue, repo *repo_model.Repository, owners []string, teamOwners []string) {
	prPoster := issue.Poster
	isAdd := true
	for _, userReviewer := range GetValidUserReviewers(ctx, owners) {
		_, _ = ReviewRequest(ctx, issue, prPoster, userReviewer, isAdd)
		// ignore the error? Seems weird, but we just won't assign them to review if they can't
	}
	for _, teamReviewer := range GetValidTeamReviewers(ctx, repo, teamOwners, prPoster) {
		_, _ = TeamReviewRequest(ctx, issue, prPoster, teamReviewer, isAdd)
		// ignore the error? Seems weird, but we just won't assign them to review if they can't
	}
}

// FindCodeownersFile gets the CODEOWNERS file from the top level or .gitea directory of the repo.
func GetCodeownersFileContents(ctx context.Context, pr *issues_model.PullRequest, gitRepo *git.Repository) []byte {
	directoryOptions := []string{"", ".gitea/"} // accepted directories to search for the CODEOWNERS file

	commit, err := gitRepo.GetCommit(pr.BaseBranch)
	if err != nil {
		// TODO: log?
		return nil
	}

	entry := GetCodeownersTreeEntry(commit, directoryOptions)
	if entry == nil {
		return nil
	}

	if entry.IsRegular() {
		gitBlob := entry.Blob()
		data, err := gitBlob.GetBlobContentBase64()
		if err != nil {
			// TODO: log?
			return nil
		}
		content, err := b64.StdEncoding.DecodeString(data)
		if err != nil {
			// TODO: log?
			return nil
		}
		return content
	}

	return nil
}

// GetCodeownersTreeEntry gets the git tree entry of the CODEOWNERS file, given an array of directories to search in
func GetCodeownersTreeEntry(commit *git.Commit, directoryOptions []string) *git.TreeEntry {
	for _, dir := range directoryOptions {
		entry, _ := commit.GetTreeEntryByPath(dir + "CODEOWNERS")
		if entry != nil {
			return entry
		}
	}
	return nil
}

// ParseCodeowners gets the users and teams that own any of the files given CODEOWNERS rules.
func ParseCodeowners(files []string, codeownersFile []byte) (users []string, teams []string, err error) {
	return []string{"user3", "not_a_real_username", "user2@g.co"}, []string{"Org1/teamA", "Org2/teamA"}, nil
	// TODO: Actually use the Parser
}

// GetValidReviewers gets the Users that actually exist and are authorized to review the pull request
func GetValidUserReviewers(ctx context.Context, userNamesOrEmails []string) (reviewers []*user_model.User) {
	reviewers = []*user_model.User{}
	for _, nameOrEmail := range userNamesOrEmails {
		var reviewer *user_model.User
		var err error
		if strings.Contains(nameOrEmail, "@") {
			reviewer, err = user_model.GetUserByEmail(ctx, nameOrEmail)
		} else {
			reviewer, err = user_model.GetUserByName(ctx, nameOrEmail)
		}
		if reviewer != nil && err == nil {
			// TODO: err = IsValidReviewRequest(ctx, reviewer, ctx.Doer, isAdd, pr.Issue, &permDoer)
			reviewers = append(reviewers, reviewer)
		}
	}
	return reviewers
}

// GetValidReviewers gets the Teams that actually exist and are authorized to review the pull request
func GetValidTeamReviewers(ctx context.Context, repo *repo_model.Repository, fullTeamNames []string, doer *user_model.User) (teamReviewers []*organization_model.Team) {
	teamReviewers = []*organization_model.Team{}

	if repo.Owner.IsOrganization() {
		for _, fullTeamName := range fullTeamNames {
			team := GetTeamFromFullName(fullTeamName, doer, ctx)
			// TODO: err = IsValidTeamReviewRequest(ctx, teamReviewer, ctx.Doer, isAdd, pr.Issue)
			if team != nil {
				teamReviewers = append(teamReviewers, team)
			}
		}
	}

	return teamReviewers
}

// GetTeamFromFullName gets the team given its full name ('{organizationName}/{teamName}')
func GetTeamFromFullName(fullTeamName string, doer *user_model.User, ctx context.Context) *organization_model.Team {
	teamNameSplit := strings.Split(fullTeamName, "/")
	if len(teamNameSplit) != 2 {
		return nil
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
		return nil
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
		team, _ = organization.GetTeam(ctx, teamName)
	}
	return team
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
