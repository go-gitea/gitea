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

// mailIssueCommentToParticipants can be used for both new issue creation and comment.
func mailIssueCommentToParticipants(issue *Issue, doer *User, mentions []string) error {
	if !setting.Service.EnableNotifyMail {
		return nil
	}

	// Mail watchers.
	watchers, err := GetWatchers(issue.RepoID)
	if err != nil {
		return fmt.Errorf("GetWatchers [%d]: %v", issue.RepoID, err)
	}

	tos := make([]string, 0, len(watchers)) // List of email addresses.
	names := make([]string, 0, len(watchers))
	for i := range watchers {
		if watchers[i].UserID == doer.ID {
			continue
		}

		to, err := GetUserByID(watchers[i].UserID)
		if err != nil {
			return fmt.Errorf("GetUserByID [%d]: %v", watchers[i].UserID, err)
		}
		if to.IsOrganization() {
			continue
		}

		tos = append(tos, to.Email)
		names = append(names, to.Name)
	}
	SendIssueCommentMail(issue, doer, tos)

	// Mail mentioned people and exclude watchers.
	names = append(names, doer.Name)
	tos = make([]string, 0, len(mentions)) // list of user names.
	for i := range mentions {
		if com.IsSliceContainsStr(names, mentions[i]) {
			continue
		}

		tos = append(tos, mentions[i])
	}
	SendIssueMentionMail(issue, doer, GetUserEmailsByNames(tos))

	return nil
}

// MailParticipants sends new issue thread created emails to repository watchers
// and mentioned people.
func (issue *Issue) MailParticipants() (err error) {
	mentions := markdown.FindAllMentions(issue.Content)
	if err = UpdateIssueMentions(x, issue.ID, mentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", issue.ID, err)
	}

	if err = mailIssueCommentToParticipants(issue, issue.Poster, mentions); err != nil {
		log.Error(4, "mailIssueCommentToParticipants: %v", err)
	}

	return nil
}
