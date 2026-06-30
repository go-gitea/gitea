// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

	// When both selectors are given, make sure they refer to the same user.
	if c.IsSet("id") && c.IsSet("username") && user.LowerName != strings.ToLower(strings.TrimSpace(c.String("username"))) {
		return fmt.Errorf("the user with id %d is %q, which does not match the provided username %q", user.ID, user.Name, c.String("username"))
	}

	totp, webAuthn, err := auth_model.DisableTwoFactor(ctx, user.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Disabled 2FA for user %q (removed %d TOTP and %d WebAuthn credential(s))\n", user.Name, totp, webAuthn)
	return nil
}
