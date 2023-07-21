// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"

	user_model "code.gitea.io/gitea/models/user"
	pwd "code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/setting"

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
	if len(c.String("password")) < setting.MinPasswordLength {
		return fmt.Errorf("Password is not long enough. Needs to be at least %d", setting.MinPasswordLength)
	}

	if !pwd.IsComplexEnough(c.String("password")) {
		return errors.New("Password does not meet complexity requirements")
	}
	pwned, err := pwd.IsPwned(context.Background(), c.String("password"))
	if err != nil {
		return err
	}
	if pwned {
		return errors.New("The password you chose is on a list of stolen passwords previously exposed in public data breaches. Please try again with a different password.\nFor more details, see https://haveibeenpwned.com/Passwords")
	}
	uname := c.String("username")
	user, err := user_model.GetUserByName(ctx, uname)
	if err != nil {
		return err
	}
	if err = user.SetPassword(c.String("password")); err != nil {
		return err
	}

	if err = user_model.UpdateUserCols(ctx, user, "passwd", "passwd_hash_algo", "salt"); err != nil {
		return err
	}

	fmt.Printf("%s's password has been successfully updated!\n", user.Name)
	return nil
}
