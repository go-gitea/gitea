// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	user_service "code.gitea.io/gitea/services/user"

	"github.com/urfave/cli/v2"
)

var microcmdUserChangePassword = &cli.Command{
	Name:   "change-password",
	Usage:  "Change a user's password",
	Action: runChangePassword,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Value:   "",
			Usage:   "The user to change password for",
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			Value:   "",
			Usage:   "New password to set for user",
		},
		&cli.BoolFlag{
			Name:  "must-change-password",
			Usage: "User must change password (can be disabled by --must-change-password=false)",
			Value: true,
		},
	},
}

func runChangePassword(c *cli.Context) error {
	if err := argsSet(c, "username", "password"); err != nil {
		return err
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	user, err := user_model.GetUserByName(ctx, c.String("username"))
	if err != nil {
		return err
	}

	opts := &user_service.UpdateAuthOptions{
		Password:           optional.Some(c.String("password")),
		MustChangePassword: optional.Some(c.Bool("must-change-password")),
	}
	if err := user_service.UpdateAuth(ctx, user, opts); err != nil {
		switch {
		case errors.Is(err, password.ErrMinLength):
			return fmt.Errorf("password is not long enough, needs to be at least %d characters", setting.MinPasswordLength)
		case errors.Is(err, password.ErrComplexity):
			return errors.New("password does not meet complexity requirements")
		case errors.Is(err, password.ErrIsPwned):
			return errors.New("the password is in a list of stolen passwords previously exposed in public data breaches, please try again with a different password, to see more details: https://haveibeenpwned.com/Passwords")
		default:
			return err
		}
	}

	fmt.Printf("%s's password has been successfully updated!\n", user.Name)
	return nil
}
