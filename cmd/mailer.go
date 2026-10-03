// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"

	"gitea.dev/modules/private"
	"gitea.dev/modules/setting"

	"github.com/urfave/cli/v3"
)

func runSendMail(ctx context.Context, c *cli.Command) error {
	setting.MustInstalled()

	subject := c.String("title")
	confirmSkipped := c.Bool("force")
	body := c.String("content")

	if !confirmSkipped {
		if len(body) == 0 {
			cprintln(c, "warning: Content is empty")
		}

		if !confirm(c.Reader, c.Writer, "Proceed with sending email? [Y/n] ") {
			cprintln(c, "The mail was not sent")
			return nil
		}
	}

	respText, extra := private.SendEmail(ctx, subject, body, nil)
	if extra.HasError() {
		return handleCliResponseExtra(extra)
	}
	cprintf(c, "Sent %s email(s) to all users\n", respText.Text)
	return nil
}
