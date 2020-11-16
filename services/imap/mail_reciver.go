// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	comment_service "code.gitea.io/gitea/services/comments"
)

// mail read query
var mailReadQueue queue.Queue

// NewContext start received mail read queue service
func NewContext() {
	if setting.MailReciveService == nil || mailReadQueue != nil {
		return
	}

	mailReadQueue = queue.CreateQueue("mail_recive", func(data ...queue.Data) {
		for _, datum := range data {
			mail := datum.(*Mail)
			if err := mail.LoadHeader([]string{"From", "To"}); err != nil {
				log.Error("fetch mail header failed: %v", err)
			}

			if len(mail.Heads["To"]) == 0 ||
				len(mail.Heads["From"]) == 0 {
				continue
			}

			if mail.Heads["To"][0].Address != setting.MailReciveService.ReciveEmail {
				continue
			}

			log.Trace("start read email from %v", mail.Heads["From"][0].String())
			if err := handleReciveEmail(mail); err != nil {
				log.Error("handleReciveEmail(): %v", err)
			}
			log.Trace("finished read email from %v", mail.Heads["From"][0].String())
		}
	}, &Mail{})
}

func handleReciveEmail(m *Mail) error {
	if err := m.LoadBody(); err != nil {
		return fmt.Errorf("m.LoadBody(): %v", err)
	}

	from := m.Heads["From"][0].Address
	doer, err := models.GetUserByEmail(from)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			return nil
		}
		return fmt.Errorf("models.GetUserByEmail(%v): %v", from, err)
	}

	// chek if it's a reply mail to an issue or pull request
	linkNode := m.Content.Find("a.reply-to")
	if linkNode.Length() != 1 {
		return nil
	}

	linkHerf, has := linkNode.First().Attr("href")
	if !has || len(linkHerf) == 0 {
		return nil
	}

	// expected link {{AppFullUrl}}/{{Owner}}/{{ReopName}}/{{issues/pulls}}/{{index}}#issuecomment-id
	link, err := url.Parse(linkHerf)
	if err != nil {
		return fmt.Errorf("url.Parse(%v): %v", linkHerf, err)
	}

	splitLink := strings.SplitN(link.Path[1:], "/", 4)
	if len(splitLink) != 4 ||
		(splitLink[2] != "pulls" && splitLink[2] != "issues") {
		return nil
	}

	repoOwner := splitLink[0]
	repoName := splitLink[1]
	issueIndex, err := strconv.ParseInt(splitLink[3], 0, 64)
	if err != nil {
		return nil
	}
	if issueIndex <= 0 {
		return nil
	}

	repo, err := models.GetRepositoryByOwnerAndName(repoOwner, repoName)
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			return nil
		}

		return fmt.Errorf("models.GetRepositoryByOwnerAndName(%v,%v): %v", repoOwner, repoName, err)
	}

	if repo.IsArchived {
		return nil
	}

	perm, err := models.GetUserRepoPermission(repo, doer)
	if err != nil {
		return fmt.Errorf("models.GetUserRepoPermission(): %v", err)
	}

	issue, err := models.GetIssueWithAttrsByIndex(repo.ID, issueIndex)
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			return nil
		}

		return fmt.Errorf("models.GetIssueWithAttrsByIndex(%v,%v): %v", repo.ID, issueIndex, err)
	}

	// check permission
	permUnit := models.UnitTypeIssues
	if issue.IsPull {
		permUnit = models.UnitTypePullRequests
	}

	if issue.IsLocked && !perm.CanWrite(permUnit) {
		return nil
	}

	if !issue.IsLocked && !perm.CanRead(permUnit) {
		return nil
	}

	comment, err := m.Content.Html()
	if err != nil {
		return fmt.Errorf("m.Content.Html(): %v", err)
	}

	_, err = comment_service.CreateIssueComment(doer,
		repo,
		issue,
		comment, nil)
	if err != nil {
		return fmt.Errorf("comment_service.CreateIssueComment(): %v", err)
	}

	_ = m.SetRead(true)

	if setting.MailReciveService.DeleteRodeMail {
		_ = m.Delete()
	}

	return nil
}
