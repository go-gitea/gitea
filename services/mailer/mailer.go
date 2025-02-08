// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"context"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	sender_service "code.gitea.io/gitea/services/mailer/sender"
	notify_service "code.gitea.io/gitea/services/notify"
)

var mailQueue *queue.WorkerPoolQueue[*sender_service.Message]

// sender sender for sending mail synchronously
var sender sender_service.Sender

// NewContext start mail queue service
func NewContext(ctx context.Context) {
	// Need to check if mailQueue is nil because in during reinstall (user had installed
	// before but switched install lock off), this function will be called again
	// while mail queue is already processing tasks, and produces a race condition.
	if setting.MailService == nil || mailQueue != nil {
		return
	}

	if setting.Service.EnableNotifyMail {
		notify_service.RegisterNotifier(NewNotifier())
	}

	switch setting.MailService.Protocol {
	case "sendmail":
		sender = &sender_service.SendmailSender{}
	case "dummy":
		sender = &sender_service.DummySender{}
	default:
		sender = &sender_service.SMTPSender{}
	}

	subjectTemplates, bodyTemplates = templates.Mailer(ctx)

	mailQueue = queue.CreateSimpleQueue(graceful.GetManager().ShutdownContext(), "mail", func(items ...*sender_service.Message) []*sender_service.Message {
		for _, msg := range items {
			gomailMsg := msg.ToMessage()
			log.Trace("New e-mail sending request %s: %s", gomailMsg.GetGenHeader("To"), msg.Info)
			if err := sender_service.Send(sender, msg); err != nil {
				log.Error("Failed to send emails %s: %s - %v", gomailMsg.GetGenHeader("To"), msg.Info, err)
			} else {
				log.Trace("E-mails sent %s: %s", gomailMsg.GetGenHeader("To"), msg.Info)
			}
		}
		return nil
	})
	if mailQueue == nil {
		log.Fatal("Unable to create mail queue")
	}
	go graceful.GetManager().RunWithCancel(mailQueue)
}

// SendAsync send emails asynchronously (make it mockable)
var SendAsync = sendAsync

func sendAsync(msgs ...*sender_service.Message) {
	if setting.MailService == nil {
		log.Error("Mailer: SendAsync is being invoked but mail service hasn't been initialized")
		return
	}

	go func() {
		for _, msg := range msgs {
			_ = mailQueue.Push(msg)
		}
	}()
}
