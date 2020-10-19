// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/services/mailer"

	"github.com/urfave/cli"
)

func runSendMail(c *cli.Context) error {
	if err := argsSet(c, "title"); err != nil {
		return err
	}

	if err := initDB(); err != nil {
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

	var emails []string
	err := models.IterateUser(func(user *models.User) error {
		emails = append(emails, user.Email)
		return nil
	})
	if err != nil {
		return errors.New("Cann't find users")
	}

	fmt.Printf("Sending %d emails", len(emails))

	mailer.NewContext()

	for _, email := range emails {
		msg := mailer.NewMessage([]string{email}, subject, body)
		mailer.SendAsync(msg)
	}

	err = mailer.FlushMessages(time.Minute * 60)
	if err != nil {
		return err
	}

	return nil
}
