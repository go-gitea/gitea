// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import "code.gitea.io/gitea/modules/log"

// MailReceiver represents mail receive service.
type MailReceiver struct {
	ReceiveEmail   string
	ReceiveBox     string
	QueueLength    int
	Host           string
	User, Passwd   string
	IsTLSEnabled   bool
	DeleteReadMail bool
}

var (
	// MailRecieveService mail receive config
	MailRecieveService *MailReceiver
)

func newMailRecieveService() {
	sec := Cfg.Section("mail_receive")
	// Check mailer setting.
	if !sec.Key("ENABLED").MustBool() {
		return
	}

	MailRecieveService = &MailReceiver{
		ReceiveEmail:   sec.Key("RECEIVE_EMAIL").String(),
		ReceiveBox:     sec.Key("RECEIVE_BOX").MustString("INBOX"),
		QueueLength:    sec.Key("READ_BUFFER_LEN").MustInt(100),
		Host:           sec.Key("HOST").String(),
		User:           sec.Key("USER").String(),
		Passwd:         sec.Key("PASSWD").String(),
		IsTLSEnabled:   sec.Key("IS_TLS_ENABLED").MustBool(true),
		DeleteReadMail: sec.Key("DELETE_READ_MAIL").MustBool(false),
	}

	log.Info("Mail Receive Service Enabled")
}
