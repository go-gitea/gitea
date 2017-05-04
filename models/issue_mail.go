// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"github.com/Unknwon/com"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"
)

func (issue *Issue) mailSubject() string {
	return fmt.Sprintf("[%s] %s (#%d)", issue.Repo.Name, issue.Title, issue.Index)
}


// mailIssueCommentToParticipants can be used only for comment.
// This function sends two list of emails:
// 1. Repository watchers and users who are participated in comments.
// 2. Users who are not in 1. but get mentioned in current issue/comment.
func mailIssueCommentToParticipants(issue *Issue, doer *User, comment *Comment, mentions []string) error {

	names, tos, err := prepareMailToParticipants(issue, doer)

	if(err != nil) {
		return fmt.Errorf("PrepareMailToParticipants: %v", err)
	}

	SendIssueCommentMail(issue, doer, comment, tos)

	// Mail mentioned people and exclude watchers.
	names = append(names, doer.Name)
	tos = make([]string, 0, len(mentions)) // list of user names.
	for i := range mentions {
		if com.IsSliceContainsStr(names, mentions[i]) {
			continue
		}

		tos = append(tos, mentions[i])
	}
	SendIssueMentionMail(issue, doer, comment, GetUserEmailsByNames(tos))

	return nil
}

// mailIssueActionToParticipants can be used for creation or pull requests.
// This function sends two list of emails:
// 1. Repository watchers and users who are participated in comments.
// 2. Users who are not in 1. but get mentioned in current issue/comment.
func mailIssueActionToParticipants(issue *Issue, doer *User, mentions []string) error {
	names, tos, err := prepareMailToParticipants(issue, doer)

	if(err != nil) {
		return fmt.Errorf("PrepareMailToParticipants: %v", err)
	}

	SendIssueActionMail(issue, doer, tos)

	// Mail mentioned people and exclude watchers.
	names = append(names, doer.Name)
	tos = make([]string, 0, len(mentions)) // list of user names.
	for i := range mentions {
		if com.IsSliceContainsStr(names, mentions[i]) {
			continue
		}

		tos = append(tos, mentions[i])
	}
	SendIssueMentionInActionMail(issue, doer, GetUserEmailsByNames(tos))

	return nil
}

// prepareMailToParticipants creates the tos and names list for an issue and the issue's creator.
func prepareMailToParticipants(issue *Issue, doer *User) (tos []string, names []string, error error)  {
	if !setting.Service.EnableNotifyMail {
		return nil, nil, nil
	}

	watchers, err := GetWatchers(issue.RepoID)
	if err != nil {
		return nil, nil, err
	}
	participants, err := GetParticipantsByIssueID(issue.ID)
	if err != nil {
		return nil, nil, err
	}

	// In case the issue poster is not watching the repository,
	// even if we have duplicated in watchers, can be safely filtered out.
	if issue.PosterID != doer.ID {
		participants = append(participants, issue.Poster)
	}

	tos = make([]string, 0, len(watchers)) // List of email addresses.
	names = make([]string, 0, len(watchers))
	for i := range watchers {
		if watchers[i].UserID == doer.ID {
			continue
		}

		to, err := GetUserByID(watchers[i].UserID)
		if err != nil {
			return nil, nil, err
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
	return tos, names, nil
}


// MailParticipants sends new issue thread created emails to repository watchers
// and mentioned people.
func (issue *Issue) MailParticipants() (err error) {
	mentions := markdown.FindAllMentions(issue.Content)
	if err = UpdateIssueMentions(x, issue.ID, mentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", issue.ID, err)
	}

	if err = mailIssueActionToParticipants(issue, issue.Poster, mentions); err != nil {
		log.Error(4, "mailIssueCommentToParticipants: %v", err)
	}

	return nil
}


