// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

const IncomingEmailTokenPlaceholder = "%{token}"

var IncomingEmail = struct {
	Enabled              bool
	ReplyToAddress       string
	Host                 string
	Port                 int
	UseTLS               bool `ini:"USE_TLS"`
	SkipTLSVerify        bool `ini:"SKIP_TLS_VERIFY"`
	Username             string
	Password             string
	Mailbox              string
	DeleteHandledMessage bool
	MaximumMessageSize   uint32
}{
	Mailbox:              "INBOX",
	DeleteHandledMessage: true,
	MaximumMessageSize:   10485760,
}

func loadIncomingEmailFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "email.incoming", &IncomingEmail)

	if !IncomingEmail.Enabled {
		return
	}

	if err := checkReplyToAddress(); err != nil {
		log.Fatal("Invalid incoming_mail.REPLY_TO_ADDRESS (%s): %v", IncomingEmail.ReplyToAddress, err)
	}
}

func checkReplyToAddress() error {
	parsed, err := mail.ParseAddress(IncomingEmail.ReplyToAddress)
	if err != nil {
		return err
	}

	if parsed.Name != "" {
		return errors.New("name must not be set")
	}

	placeholderCount := strings.Count(IncomingEmail.ReplyToAddress, IncomingEmailTokenPlaceholder)
	userPart, _, _ := strings.Cut(IncomingEmail.ReplyToAddress, "@")
	if placeholderCount != 1 || !strings.Contains(userPart, IncomingEmailTokenPlaceholder) {
		return fmt.Errorf("%s must appear in the user part of the address (before the @)", IncomingEmailTokenPlaceholder)
	}
	return nil
}
