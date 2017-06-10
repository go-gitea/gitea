// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"io"
	"os/exec"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	gomail "gopkg.in/gomail.v2"
)

// Sender sendmail mail sender
type sendmailSender struct {
	sender gomail.Sender
}

func newSendmailSender() (Sender, error) {
	s := &sendmailSender{}
	s.sender = gomail.SendFunc(s.send)

	return s, nil
}

func (s *sendmailSender) Close() error {
	return nil
}

// Send the message synchronous.
func (s *sendmailSender) Send(msg *Message) error {
	return gomail.Send(s.sender, msg.Message)
}

// send email.
func (s *sendmailSender) send(from string, to []string, msg io.WriterTo) error {
	args := []string{"-F", from, "-i"}
	args = append(args, to...)
	log.Trace("Sending with: %s %v", setting.MailService.SendmailPath, args)
	cmd := exec.Command(setting.MailService.SendmailPath, args...)

	// Stdin Pipe for message content.
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	// Execute the command.
	err = cmd.Start()
	if err != nil {
		return err
	}

	// Write the message to the pipe and wait for the command to finish.
	// We MUST close the pipe or sendmail will hang waiting for more of the message
	// Also we should wait on our sendmail command even if something fails
	_, err = msg.WriteTo(pipe)
	closeError := pipe.Close()
	waitError := cmd.Wait()
	if err != nil {
		return err
	} else if closeError != nil {
		return closeError
	} else {
		return waitError
	}
}
