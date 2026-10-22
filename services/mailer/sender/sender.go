// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"errors"
	"io"
)

type Sender interface {
	Send(from string, to []string, msg io.WriterTo) error
}

var Send = send

func send(sender Sender, msg *Message) error {
	m := msg.ToMessage()
	froms := m.GetFrom()
	to, err := m.GetRecipients()
	if err != nil {
		return err
	}

	// TODO: implement sending from multiple addresses
	if len(froms) == 0 {
		// FIXME: no idea why sometimes the "froms" can be empty, need to figure out the root problem
		return errors.New("no FROM specified")
	}
	return sender.Send(froms[0].Address, to, m)
}
