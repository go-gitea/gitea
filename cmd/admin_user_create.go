// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	pwd "code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"

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
			Name:               "must-change-password",
			Usage:              "User must change password after initial login, defaults to true for all users except the first one (can be disabled by --must-change-password=false)",
			DisableDefaultText: true,
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
	// this command highly depends on the many setting options (create org, visibility, etc.), so it must have a full setting load first
	// duplicate setting loading should be safe at the moment, but it should be refactored & improved in the future.
	setting.LoadSettings()

	if err := argsSet(c, "email"); err != nil {
		return err
	}

	if c.IsSet("name") && c.IsSet("username") {
		return errors.New("cannot set both --name and --username flags")
	}
	if !c.IsSet("name") && !c.IsSet("username") {
		return errors.New("one of --name or --username flags must be set")
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

	ctx := c.Context
	if !setting.IsInTesting {
		// FIXME: need to refactor the "installSignals/initDB" related code later
		// it doesn't make sense to call it in (almost) every command action function
		var cancel context.CancelFunc
		ctx, cancel = installSignals()
		defer cancel()
		if err := initDB(ctx); err != nil {
			return err
		}
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

	isAdmin := c.Bool("admin")
	mustChangePassword := true // always default to true
	if c.IsSet("must-change-password") {
		// if the flag is set, use the value provided by the user
		mustChangePassword = c.Bool("must-change-password")
	} else {
		// check whether there are users in the database
		hasUserRecord, err := db.IsTableNotEmpty(&user_model.User{})
		if err != nil {
			return fmt.Errorf("IsTableNotEmpty: %w", err)
		}
		if !hasUserRecord {
			// if this is the first one being created, don't force to change password (keep the old behavior)
			mustChangePassword = false
		}
	}

	restricted := optional.None[bool]()

	if c.IsSet("restricted") {
		restricted = optional.Some(c.Bool("restricted"))
	}

	// default user visibility in app.ini
	visibility := setting.Service.DefaultUserVisibilityMode

	u := &user_model.User{
		Name:               username,
		Email:              c.String("email"),
		Passwd:             password,
		IsAdmin:            isAdmin,
		MustChangePassword: mustChangePassword,
		Visibility:         visibility,
	}

	overwriteDefault := &user_model.CreateUserOverwriteOptions{
		IsActive:     optional.Some(true),
		IsRestricted: restricted,
	}

	if err := user_model.CreateUser(ctx, u, &user_model.Meta{}, overwriteDefault); err != nil {
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
