// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var (
	daemon *Daemon
)

// NewContext start mail queue service
func NewContext() {
	// TODO: Is this still present? Why is it anyway possible that this initializer is called mutliple times?!
	// Need to check if the daemon is nil because in during reinstall (user had installed
	// before but swithed install lock off), this function will be called again
	// while mail queue is already processing tasks, and produces a race condition.
	if setting.MailService == nil || daemon != nil {
		return
	}

	var err error
	daemon, err = NewDaemon()
	if err != nil {
		log.Fatal(4, "Failed to initialize mail daemon: %v", err)
	}
}

// TODO: Call this on application exit.
// CloseContext closes the mail queue service and releases all routines.
func CloseContext() {
	daemon.Close()
}

// SendAsync send mail asynchronous.
func SendAsync(msg *Message) {
	daemon.SendAsync(msg)
}
