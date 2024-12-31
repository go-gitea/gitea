// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/mail"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

var IncomingEmail = struct {
	Enabled              bool
	ReplyToAddress       string
	TokenPlaceholder     string `ini:"-"`
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
	TokenPlaceholder:     "%{token}",
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
		return fmt.Errorf("name must not be set")
	}

	c := strings.Count(IncomingEmail.ReplyToAddress, IncomingEmail.TokenPlaceholder)
	switch c {
	case 0:
		return fmt.Errorf("%s must appear in the user part of the address (before the @)", IncomingEmail.TokenPlaceholder)
	case 1:
	default:
		return fmt.Errorf("%s must appear only once", IncomingEmail.TokenPlaceholder)
	}

	parts := strings.Split(IncomingEmail.ReplyToAddress, "@")
	if !strings.Contains(parts[0], IncomingEmail.TokenPlaceholder) {
		return fmt.Errorf("%s must appear in the user part of the address (before the @)", IncomingEmail.TokenPlaceholder)
	}

	return nil
}
