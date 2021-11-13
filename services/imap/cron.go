// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"code.gitea.io/gitea/modules/setting"
)

var (
	c *Client
)

// FetchAllUnreadMails fetch all unread mails
func FetchAllUnreadMails() (err error) {
	if c == nil {
		c, err = NewImapClient(ClientInitOpt{
			Addr:     setting.MailRecieveService.Host,
			UserName: setting.MailRecieveService.User,
			Passwd:   setting.MailRecieveService.Passwd,
			IsTLS:    setting.MailRecieveService.IsTLSEnabled,
		})
		if err != nil {
			return
		}
	}

	if mailReadQueue != nil && !mailReadQueue.IsEmpty() {
		return
	}

	mails, err := c.GetUnreadMails(setting.MailRecieveService.ReceiveBox, 100)
	if err != nil {
		return
	}

	for _, mail := range mails {
		err = mailReadQueue.Push(mail)
		if err != nil {
			return err
		}
	}

	return nil
}
