// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
)

func fallbackMailSubject(issue *models.Issue) string {
	return fmt.Sprintf("[%s] %s (#%d)", issue.Repo.FullName(), issue.Title, issue.Index)
}

// mailIssueCommentToParticipants can be used for both new issue creation and comment.
// This function sends two list of emails:
// 1. Repository watchers and users who are participated in comments.
// 2. Users who are not in 1. but get mentioned in current issue/comment.
func mailIssueCommentToParticipants(issue *models.Issue, doer *models.User, actionType models.ActionType, content string, comment *models.Comment, mentions []string) error {

	// =========== Repo watchers ===========
	// *Watch
	rwatchers, err := models.GetWatchers(issue.RepoID)
	if err != nil {
		return fmt.Errorf("GetWatchers(%d): %v", issue.RepoID, err)
	}
	watcherids := make([]int64, len(rwatchers))
	for i := range rwatchers {
		watcherids[i] = rwatchers[i].UserID
	}
	watchers, err := models.GetUsersByIDs(watcherids)
	if err != nil {
		return fmt.Errorf("GetUsersByIDs(%d): %v", issue.RepoID, err)
	}

	// =========== Issue watchers ===========
	// IssueWatchList
	iwl, err := models.GetIssueWatchers(issue.ID)
	if err != nil {
		return fmt.Errorf("GetIssueWatchers(%d): %v", issue.ID, err)
	}
	// UserList ([]*User)
	iwatchers, err := iwl.LoadWatchUsers()
	if err != nil {
		return fmt.Errorf("GetIssueWatchers(%d): %v", issue.ID, err)
	}

	// =========== Participants (i.e. commenters, reviewers) ===========
	// []*User
	participants, err := models.GetParticipantsByIssueID(issue.ID)
	if err != nil {
		return fmt.Errorf("GetParticipantsByIssueID(%d): %v", issue.ID, err)
	}

	// =========== Assignees ===========
	// []*User
	assignees, err := models.GetAssigneesByIssue(issue)
	if err != nil {
		return err
	}

	// =========== Original poster ===========
	// *User
	err = issue.LoadPoster()
	if err != nil {
		return fmt.Errorf("LoadPoster(%d): %v", issue.PosterID, err)
	}

	recipients := make([]*models.User, 0, 10)
	visited := make(map[string]bool, 10)

	// Avoid mailing the doer
	visited[doer.LowerName] = true

	// Normalize all aditions to make all the relevant checks
	recipients = addUniqueUsers(visited, recipients, []*models.User{issue.Poster})
	recipients = addUniqueUsers(visited, recipients, watchers)
	recipients = addUniqueUsers(visited, recipients, iwatchers)
	recipients = addUniqueUsers(visited, recipients, participants)
	recipients = addUniqueUsers(visited, recipients, assignees)

	tos := make([]string, 0, len(recipients)) // List of email addresses.
	for _, to := range recipients {
		tos = append(tos, to.Email)
	}

	if err := issue.LoadRepo(); err != nil {
		return err
	}

	for _, to := range tos {
		SendIssueCommentMail(issue, doer, actionType, content, comment, []string{to})
	}

	// Mail mentioned people and exclude previous recipients
	tos = make([]string, 0, len(mentions)) // mentions come as a list of user names
	for _, mention := range mentions {
		if _, ok := visited[mention]; !ok {
			visited[mention] = true
			tos = append(tos, mention)
		}
	}

	emails := models.GetUserEmailsByNames(tos)

	for _, to := range emails {
		SendIssueMentionMail(issue, doer, actionType, content, comment, []string{to})
	}

	return nil
}

func addUniqueUsers(visited map[string]bool, current []*models.User, list []*models.User) []*models.User {
	for _, u := range list {
		if _, ok := visited[u.LowerName]; !ok &&
			!u.IsOrganization() &&
			u.EmailNotifications() == models.EmailNotificationsEnabled &&
			!u.ProhibitLogin &&
			u.IsActive {
			visited[u.LowerName] = true
			current = append(current, u)
		}
	}
	return current
}

// MailParticipants sends new issue thread created emails to repository watchers
// and mentioned people.
func MailParticipants(issue *models.Issue, doer *models.User, opType models.ActionType) error {
	return mailParticipants(models.DefaultDBContext(), issue, doer, opType)
}

func mailParticipants(ctx models.DBContext, issue *models.Issue, doer *models.User, opType models.ActionType) (err error) {
	rawMentions := references.FindAllMentionsMarkdown(issue.Content)
	userMentions, err := issue.ResolveMentionsByVisibility(ctx, doer, rawMentions)
	if err != nil {
		return fmt.Errorf("ResolveMentionsByVisibility [%d]: %v", issue.ID, err)
	}
	if err = models.UpdateIssueMentions(ctx, issue.ID, userMentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", issue.ID, err)
	}
	mentions := make([]string, len(userMentions))
	for i, u := range userMentions {
		mentions[i] = u.LowerName
	}
	if err = mailIssueCommentToParticipants(issue, doer, opType, issue.Content, nil, mentions); err != nil {
		log.Error("mailIssueCommentToParticipants: %v", err)
	}
	return nil
}
