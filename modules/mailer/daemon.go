// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"fmt"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
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

	// Our sender creation function.
	createSender := createSenderFunc()

	// Create a sender for each mail routine.
	for i := 0; i < routines; i++ {
		s, err := createSender()
		if err != nil {
			return nil, err
		}

		go d.processMailQueue(s)
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

// SendAsync send mail asynchronous.
func (d *Daemon) SendAsync(msg *Message) {
	// TODO: don't start new goroutines. Drop mails if the channel is flooded.
	// TODO: Increase the channel size.
	go func() {
		// Don't block if closed.
		select {
		case <-d.closeChan:
		case d.mailQueue <- msg:
		}
	}()
}

func (d *Daemon) processMailQueue(s Sender) {
	var err error

	for {
		select {
		case <-d.closeChan:
			if err = s.Close(); err != nil {
				log.Error(3, "Failed to close mail sender connection: %v", err)
			}
			return

		case msg := <-d.mailQueue:
			log.Trace("New e-mails sending request %s: %s", msg.GetHeader("To"), msg.Info)
			if err = s.Send(msg); err != nil {
				log.Error(3, "Failed to send emails %s: %s - %v", msg.GetHeader("To"), msg.Info, err)
			} else {
				log.Trace("E-mails sent %s: %s", msg.GetHeader("To"), msg.Info)
			}

		// TODO: Reuse the timer.
		// Close the mail server connection if no email was sent in
		// the last 30 seconds.
		case <-time.After(30 * time.Second):
			if err = s.Close(); err != nil {
				log.Error(3, "Failed to close mail sender connection: %v", err)
			}
		}
	}
}
