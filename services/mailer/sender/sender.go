// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gopkg.in/gomail.v2"
)

type Sender gomail.Sender

var Send = send

func send(sender Sender, msgs ...*Message) error {
	if setting.MailService == nil {
		log.Error("Mailer: Send is being invoked but mail service hasn't been initialized")
		return nil
	}
	goMsgs := []*gomail.Message{}
	for _, msg := range msgs {
		goMsgs = append(goMsgs, msg.ToMessage())
	}
	return gomail.Send(sender, goMsgs...)
}
