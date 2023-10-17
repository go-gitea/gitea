// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"

	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	pwd "code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli/v2"
)

var microcmdUserCreate = &cli.Command{
	Name:   "create",
	Usage:  "Create a new user in database",
	Action: runCreateUser,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "name",
			Usage: "Username. DEPRECATED: use username instead",
		},
		&cli.StringFlag{
			Name:  "username",
			Usage: "Username",
		},
		&cli.StringFlag{
			Name:  "password",
			Usage: "User password",
		},
		&cli.StringFlag{
			Name:  "email",
			Usage: "User email address",
		},
		&cli.BoolFlag{
			Name:  "admin",
			Usage: "User is an admin",
		},
		&cli.BoolFlag{
			Name:  "random-password",
			Usage: "Generate a random password for the user",
		},
		&cli.BoolFlag{
			Name:  "must-change-password",
			Usage: "Set this option to false to prevent forcing the user to change their password after initial login, (Default: true)",
		},
		&cli.IntFlag{
			Name:  "random-password-length",
			Usage: "Length of the random password to be generated",
			Value: 12,
		},
		&cli.BoolFlag{
			Name:  "access-token",
			Usage: "Generate access token for the user",
		},
		&cli.BoolFlag{
			Name:  "restricted",
			Usage: "Make a restricted user account",
		},
	},
}

func runCreateUser(c *cli.Context) error {
	if err := argsSet(c, "email"); err != nil {
		return err
	}

	if c.IsSet("name") && c.IsSet("username") {
		return errors.New("Cannot set both --name and --username flags")
	}
	if !c.IsSet("name") && !c.IsSet("username") {
		return errors.New("One of --name or --username flags must be set")
	}

	if c.IsSet("password") && c.IsSet("random-password") {
		return errors.New("cannot set both -random-password and -password flags")
	}

	var username string
	if c.IsSet("username") {
		username = c.String("username")
	} else {
		username = c.String("name")
		_, _ = fmt.Fprintf(c.App.ErrWriter, "--name flag is deprecated. Use --username instead.\n")
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	var password string
	if c.IsSet("password") {
		password = c.String("password")
	} else if c.IsSet("random-password") {
		var err error
		password, err = pwd.Generate(c.Int("random-password-length"))
		if err != nil {
			return err
		}
		fmt.Printf("generated random password is '%s'\n", password)
	} else {
		return errors.New("must set either password or random-password flag")
	}

	// always default to true
	changePassword := true

	// If this is the first user being created.
	// Take it as the admin and don't force a password update.
	if n := user_model.CountUsers(ctx, nil); n == 0 {
		changePassword = false
	}

	if c.IsSet("must-change-password") {
		changePassword = c.Bool("must-change-password")
	}

	restricted := util.OptionalBoolNone

	if c.IsSet("restricted") {
		restricted = util.OptionalBoolOf(c.Bool("restricted"))
	}

	// default user visibility in app.ini
	visibility := setting.Service.DefaultUserVisibilityMode

	u := &user_model.User{
		Name:               username,
		Email:              c.String("email"),
		Passwd:             password,
		IsAdmin:            c.Bool("admin"),
		MustChangePassword: changePassword,
		Visibility:         visibility,
	}

	overwriteDefault := &user_model.CreateUserOverwriteOptions{
		IsActive:     util.OptionalBoolTrue,
		IsRestricted: restricted,
	}

	if err := user_model.CreateUser(ctx, u, overwriteDefault); err != nil {
		return fmt.Errorf("CreateUser: %w", err)
	}

	if c.Bool("access-token") {
		t := &auth_model.AccessToken{
			Name: "gitea-admin",
			UID:  u.ID,
		}

		if err := auth_model.NewAccessToken(ctx, t); err != nil {
			return err
		}

		fmt.Printf("Access token was successfully created... %s\n", t.Token)
	}

	fmt.Printf("New user '%s' has been successfully created!\n", username)
	return nil
}
