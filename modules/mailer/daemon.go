// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"fmt"
	"sync"

	"code.gitea.io/gitea/modules/setting"
)

type Daemon struct {
	mailQueue chan *Message

	closeMutex sync.Mutex
	closeChan  chan struct{}
}

func NewDaemon() (*Daemon, error) {
	queueLen := setting.MailService.QueueLength
	routines := 1 // TODO: Get this from the settings.

	// Validate input.
	if queueLen < 0 {
		return nil, fmt.Errorf("mail daemon: invalid queue length: %v", queueLen)
	} else if routines < 1 {
		return nil, fmt.Errorf("mail daemon: invalid routines: %v", routines)
	}

	d := &Daemon{
		mailQueue: make(chan *Message, queueLen),
		closeChan: make(chan struct{}),
	}

	// Start the send mail routines.
	for i := 0; i < routines; i++ {
		if setting.MailService.UseSendmail {
			_, err := newSendmailSender(d.mailQueue, d.closeChan)
			if err != nil {
				return nil, err
			}
		} else {
			_, err := newSMTPSender(d.mailQueue, d.closeChan)
			if err != nil {
				return nil, err
			}
		}
	}

	return d, nil
}

// IsClosed returns a boolean indicating if the daemon is closed.
// This method is thread-safe.
func (d *Daemon) IsClosed() bool {
	select {
	case <-d.closeChan:
		return true
	default:
		return false
	}
}

// Close the daemon and top all routines.
// This method is thread-safe.
func (d *Daemon) Close() {
	d.closeMutex.Lock()
	defer d.closeMutex.Unlock()

	// Check if already closed.
	if d.IsClosed() {
		return
	}

	// Release routines.
	close(d.closeChan)
}

// SendSync send mail synchronous.
func (d *Daemon) SendSync(msg *Message) {
	// Don't block if closed.
	select {
	case <-d.closeChan:
	case d.mailQueue <- msg:
	}
}

// SendAsync send mail asynchronous.
func (d *Daemon) SendAsync(msg *Message) {
	// TODO: don't start new goroutines. Drop mails if the channel is flooded.
	// TODO: Increase the channel size.
	go d.SendSync(msg)
}
