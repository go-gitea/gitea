// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import "code.gitea.io/gitea/modules/setting"

type Sender interface {
	// Send the message synchronous. The connection must be opened if required.
	Send(msg *Message) (err error)

	// Close the connection if open.
	// This method can be called multiple times.
	Close() error
}

// createSenderFunc returns the function to create the actual sender.
func createSenderFunc() (f func() (Sender, error)) {
	if setting.MailService.UseSendmail {
		f = newSendmailSender
	} else {
		f = newSMTPSender
	}
	return
}
