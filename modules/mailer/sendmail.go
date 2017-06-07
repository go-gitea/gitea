// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"io"
	"os/exec"

	gomail "gopkg.in/gomail.v2"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Sender sendmail mail sender
type sendmailSender struct {
	queue     chan *Message
	closeChan chan struct{}
}

func newSendmailSender(queue chan *Message, closeChan chan struct{}) (s *sendmailSender, err error) {
	s = &sendmailSender{
		queue:     queue,
		closeChan: closeChan,
	}

	// Start the sender routine.
	go s.processMailQueue()

	return
}

func (s *sendmailSender) processMailQueue() {
	var err error
	sender := gomail.SendFunc(s.send)

	for {
		select {
		case <-s.closeChan:
			return

		case msg := <-s.queue:
			log.Trace("New e-mails sending request %s: %s", msg.GetHeader("To"), msg.Info)
			if err = gomail.Send(sender, msg.Message); err != nil {
				log.Error(3, "Failed to send emails %s: %s - %v", msg.GetHeader("To"), msg.Info, err)
			} else {
				log.Trace("E-mails sent %s: %s", msg.GetHeader("To"), msg.Info)
			}
		}
	}
}

// TODO: Cleanup the error handing and use defers.
// send email.
func (s *sendmailSender) send(from string, to []string, msg io.WriterTo) error {
	var err error
	var closeError error
	var waitError error

	args := []string{"-F", from, "-i"}
	args = append(args, to...)
	log.Trace("Sending with: %s %v", setting.MailService.SendmailPath, args)
	cmd := exec.Command(setting.MailService.SendmailPath, args...)
	pipe, err := cmd.StdinPipe()

	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	_, err = msg.WriteTo(pipe)

	// we MUST close the pipe or sendmail will hang waiting for more of the message
	// Also we should wait on our sendmail command even if something fails
	closeError = pipe.Close()
	waitError = cmd.Wait()
	if err != nil {
		return err
	} else if closeError != nil {
		return closeError
	} else {
		return waitError
	}
}
