// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

type Sender interface {
	// Send the message synchronous. The connection must be opened if required.
	Send(msg *Message) (err error)

	// Close the connection if open.
	// This method can be called multiple times.
	Close() error
}
