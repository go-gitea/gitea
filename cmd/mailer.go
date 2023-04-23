// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

func runSendMail(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setting.InitProviderFromExistingFile()
	setting.LoadCommonSettings()

	if err := argsSet(c, "title"); err != nil {
		return err
	}

	subject := c.String("title")
	confirmSkiped := c.Bool("force")
	body := c.String("content")

	if !confirmSkiped {
		if len(body) == 0 {
			fmt.Print("warning: Content is empty")
		}

		fmt.Print("Proceed with sending email? [Y/n] ")
		isConfirmed, err := confirm()
		if err != nil {
			return err
		} else if !isConfirmed {
			fmt.Println("The mail was not sent")
			return nil
		}
	}

	respText, extra := private.SendEmail(ctx, subject, body, nil)
	if extra.HasError() {
		return handleCliResponseExtra(extra)
	}
	_, _ = fmt.Printf("Sent %s email(s) to all users\n", respText)
	return nil
}
