// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"io"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

type Sender interface {
	Send(from string, to []string, msg io.WriterTo) error
}

var Send = send

func send(sender Sender, msgs ...*Message) error {
	if setting.MailService == nil {
		log.Error("Mailer: Send is being invoked but mail service hasn't been initialized")
		return nil
	}
	for _, msg := range msgs {
		m := msg.ToMessage()
		froms := m.GetFrom()
		to, err := m.GetRecipients()
		if err != nil {
			return err
		}

		// TODO: implement sending from multiple addresses
		if err := sender.Send(froms[0].Address, to, m); err != nil {
			return err
		}
	}
	return nil
}
