// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"

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
			fmt.Println("warning: Content is empty")
		}

		if !confirm(c.Reader, c.Writer, "Proceed with sending email? [Y/n] ") {
			fmt.Println("The mail was not sent")
			return nil
		}
	}

	respText, extra := private.SendEmail(ctx, subject, body, nil)
	if extra.HasError() {
		return handleCliResponseExtra(extra)
	}
	_, _ = fmt.Printf("Sent %s email(s) to all users\n", respText.Text)
	return nil
}
