// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"bytes"
	"io"

	"code.gitea.io/gitea/modules/log"
)

// DummySender Sender sendmail mail sender
type DummySender struct{}

var _ Sender = &DummySender{}

// Send send email
func (s *DummySender) Send(from string, to []string, msg io.WriterTo) error {
	buf := bytes.Buffer{}
	if _, err := msg.WriteTo(&buf); err != nil {
		return err
	}
	log.Debug("Mail From: %s To: %v Body: %s", from, to, buf.String())
	return nil
}
