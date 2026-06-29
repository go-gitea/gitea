// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"

	auth_model "gitea.dev/models/auth"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"

	"github.com/urfave/cli/v3"
)

func microcmdUserDisableTwoFactor() *cli.Command {
	return &cli.Command{
		Name:  "disable-2fa",
		Usage: "Disable two-factor authentication for a user",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "username",
				Aliases: []string{"u"},
				Usage:   "Username of the user to disable 2FA for",
			},
			&cli.Int64Flag{
				Name:  "id",
				Usage: "ID of the user to disable 2FA for",
			},
		},
		Action: runDisableTwoFactor,
	}
}

func runDisableTwoFactor(ctx context.Context, c *cli.Command) error {
	if !c.IsSet("id") && !c.IsSet("username") {
		return errors.New("either --id or --username must be provided")
	}
	if c.IsSet("id") && c.IsSet("username") {
		return errors.New("provide exactly one of --id or --username")
	}

	if !setting.IsInTesting {
		if err := initDB(ctx); err != nil {
			return err
		}
	}

	var user *user_model.User
	var err error
	if c.IsSet("id") {
		user, err = user_model.GetUserByID(ctx, c.Int64("id"))
	} else {
		user, err = user_model.GetUserByName(ctx, c.String("username"))
	}
	if err != nil {
		return err
	}

	removed, err := auth_model.DisableTwoFactor(ctx, user.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Disabled 2FA for user %q (removed %d credential(s))\n", user.Name, removed)
	return nil
}
