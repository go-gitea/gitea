// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import "code.gitea.io/gitea/modules/setting"

// Sender defines an mail sender backend implementation interface.
type Sender interface {
	// Send the message synchronous. The connection must be opened if required.
	Send(msg *Message) (err error)

	// Close the connection if open.
	// This method can be called multiple times.
	Close() error
}

// createSender creates the actual sender value, depending on the choosen sender backend.
func createSender() (Sender, error) {
	if setting.MailService.UseSendmail {
		return newSendmailSender()
	} else {
		return newSMTPSender()
	}
}
