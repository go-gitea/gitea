package notification

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

type notificationService struct {
	issueQueue chan *models.Issue
}

var (
	// Service is the notification service
	Service = &notificationService{
		issueQueue: make(chan *models.Issue, 100),
	}
)

func init() {
	go Service.Run()
}

func (ns *notificationService) Run() {
	for {
		select {
		case issue := <-ns.issueQueue:
			if err := models.CreateOrUpdateIssueNotifications(issue); err != nil {
				log.Error(4, "Was unable to create issue notification: %v", err)
			}
		}
	}
}

func (ns *notificationService) NotifyIssue(issue *models.Issue) {
	ns.issueQueue <- issue
}
