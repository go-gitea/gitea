// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"github.com/Unknwon/com"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
)

func (issue *Issue) mailSubject() string {
	return fmt.Sprintf("[%s] %s (#%d)", issue.Repo.FullName(), issue.Title, issue.Index)
}

// mailIssueCommentToParticipants can be used for both new issue creation and comment.
// This function sends two list of emails:
// 1. Repository watchers and users who are participated in comments.
// 2. Users who are not in 1. but get mentioned in current issue/comment.
func mailIssueCommentToParticipants(e Engine, issue *Issue, doer *User, content string, comment *Comment, mentions []string) error {
	if !setting.Service.EnableNotifyMail {
		return nil
	}

	watchers, err := getWatchers(e, issue.RepoID)
	if err != nil {
		return fmt.Errorf("getWatchers [repo_id: %d]: %v", issue.RepoID, err)
	}
	participants, err := getParticipantsByIssueID(e, issue.ID)
	if err != nil {
		return fmt.Errorf("getParticipantsByIssueID [issue_id: %d]: %v", issue.ID, err)
	}

	// In case the issue poster is not watching the repository and is active,
	// even if we have duplicated in watchers, can be safely filtered out.
	err = issue.loadPoster(e)
	if err != nil {
		return fmt.Errorf("GetUserByID [%d]: %v", issue.PosterID, err)
	}
	if issue.PosterID != doer.ID && issue.Poster.IsActive && !issue.Poster.ProhibitLogin {
		participants = append(participants, issue.Poster)
	}

	// Assignees must receive any communications
	assignees, err := getAssigneesByIssue(e, issue)
	if err != nil {
		return err
	}

	for _, assignee := range assignees {
		if assignee.ID != doer.ID {
			participants = append(participants, assignee)
		}
	}

	tos := make([]string, 0, len(watchers)) // List of email addresses.
	names := make([]string, 0, len(watchers))
	for i := range watchers {
		if watchers[i].UserID == doer.ID {
			continue
		}

		to, err := getUserByID(e, watchers[i].UserID)
		if err != nil {
			return fmt.Errorf("GetUserByID [%d]: %v", watchers[i].UserID, err)
		}
		if to.IsOrganization() {
			continue
		}

		tos = append(tos, to.Email)
		names = append(names, to.Name)
	}
	for i := range participants {
		if participants[i].ID == doer.ID {
			continue
		} else if com.IsSliceContainsStr(names, participants[i].Name) {
			continue
		}

		tos = append(tos, participants[i].Email)
		names = append(names, participants[i].Name)
	}

	if err := issue.loadRepo(e); err != nil {
		return err
	}

	for _, to := range tos {
		SendIssueCommentMail(issue, doer, content, comment, []string{to})
	}

	// Mail mentioned people and exclude watchers.
	names = append(names, doer.Name)
	tos = make([]string, 0, len(mentions)) // list of user names.
	for i := range mentions {
		if com.IsSliceContainsStr(names, mentions[i]) {
			continue
		}

		tos = append(tos, mentions[i])
	}

	emails := getUserEmailsByNames(e, tos)

	for _, to := range emails {
		SendIssueMentionMail(issue, doer, content, comment, []string{to})
	}

	return nil
}

// MailParticipants sends new issue thread created emails to repository watchers
// and mentioned people.
func (issue *Issue) MailParticipants(doer *User, opType ActionType) (err error) {
	return issue.mailParticipants(x, doer, opType)
}

func (issue *Issue) mailParticipants(e Engine, doer *User, opType ActionType) (err error) {
	rawMentions := markup.FindAllMentions(issue.Content)
	userMentions, err := issue.ResolveMentionsByVisibility(e, doer, rawMentions)
	if err != nil {
		return fmt.Errorf("ResolveMentionsByVisibility [%d]: %v", issue.ID, err)
	}
	if err = UpdateIssueMentions(e, issue.ID, userMentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", issue.ID, err)
	}
	mentions := make([]string, len(userMentions))
	for i, u := range userMentions {
		mentions[i] = u.LowerName
	}

	if len(issue.Content) > 0 {
		if err = mailIssueCommentToParticipants(e, issue, doer, issue.Content, nil, mentions); err != nil {
			log.Error("mailIssueCommentToParticipants: %v", err)
		}
	}

	switch opType {
	case ActionCreateIssue, ActionCreatePullRequest:
		if len(issue.Content) == 0 {
			ct := fmt.Sprintf("Created #%d.", issue.Index)
			if err = mailIssueCommentToParticipants(e, issue, doer, ct, nil, mentions); err != nil {
				log.Error("mailIssueCommentToParticipants: %v", err)
			}
		}
	case ActionCloseIssue, ActionClosePullRequest:
		ct := fmt.Sprintf("Closed #%d.", issue.Index)
		if err = mailIssueCommentToParticipants(e, issue, doer, ct, nil, mentions); err != nil {
			log.Error("mailIssueCommentToParticipants: %v", err)
		}
	case ActionReopenIssue, ActionReopenPullRequest:
		ct := fmt.Sprintf("Reopened #%d.", issue.Index)
		if err = mailIssueCommentToParticipants(e, issue, doer, ct, nil, mentions); err != nil {
			log.Error("mailIssueCommentToParticipants: %v", err)
		}
	}

	return nil
}
