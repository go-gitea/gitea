// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
)

// SendmailSender Sender sendmail mail sender
type SendmailSender struct{}

var _ Sender = &SendmailSender{}

// Send send email
func (s *SendmailSender) Send(from string, to []string, msg io.WriterTo) error {
	var err error
	var closeError error
	var waitError error

	envelopeFrom := from
	if setting.MailService.OverrideEnvelopeFrom {
		envelopeFrom = setting.MailService.EnvelopeFrom
	}

	args := []string{"-f", envelopeFrom, "-i"}
	args = append(args, setting.MailService.SendmailArgs...)
	args = append(args, to...)
	log.Trace("Sending with: %s %v", setting.MailService.SendmailPath, args)

	desc := fmt.Sprintf("SendMail: %s %v", setting.MailService.SendmailPath, args)

	ctx, _, finished := process.GetManager().AddContextTimeout(graceful.GetManager().HammerContext(), setting.MailService.SendmailTimeout, desc)
	defer finished()

	cmd := exec.CommandContext(ctx, setting.MailService.SendmailPath, args...)
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	process.SetSysProcAttribute(cmd)

	if err = cmd.Start(); err != nil {
		_ = pipe.Close()
		return err
	}

	if setting.MailService.SendmailConvertCRLF {
		buf := &strings.Builder{}
		_, err = msg.WriteTo(buf)
		if err == nil {
			_, err = strings.NewReplacer("\r\n", "\n").WriteString(pipe, buf.String())
		}
	} else {
		_, err = msg.WriteTo(pipe)
	}

	// we MUST close the pipe or sendmail will hang waiting for more of the message
	// Also we should wait on our sendmail command even if something fails
	closeError = pipe.Close()
	waitError = cmd.Wait()
	if err != nil {
		return err
	} else if closeError != nil {
		return closeError
	}
	return waitError
}
