// Copyright 2021 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
)

func TestMain(m *testing.M) {
	setting.MailRecieveService = &setting.MailReceiver{
		ReceiveEmail:   "receive@gitea.io",
		ReceiveBox:     "INBOX",
		QueueLength:    100,
		Host:           "127.0.0.1:1313",
		User:           "receive@gitea.io",
		Passwd:         "123456",
		IsTLSEnabled:   false,
		DeleteReadMail: true,
	}

	c = new(Client)

	c.UserName = setting.MailRecieveService.User
	c.Passwd = setting.MailRecieveService.Passwd
	c.Addr = setting.MailRecieveService.Host
	c.IsTLS = setting.MailRecieveService.IsTLSEnabled
	c.Client = new(testIMAPClient)

	unittest.MainTest(m, filepath.Join("..", ".."))
}
