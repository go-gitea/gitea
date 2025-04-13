// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/storage"
	user_service "code.gitea.io/gitea/services/user"

	"github.com/urfave/cli/v2"
)

var microcmdUserDelete = &cli.Command{
	Name:  "delete",
	Usage: "Delete specific user by id, name or email",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:  "id",
			Usage: "ID of user of the user to delete",
		},
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "Username of the user to delete",
		},
		&cli.StringFlag{
			Name:    "email",
			Aliases: []string{"e"},
			Usage:   "Email of the user to delete",
		},
		&cli.BoolFlag{
			Name:  "purge",
			Usage: "Purge user, all their repositories, organizations and comments",
		},
	},
	Action: runDeleteUser,
}

func runDeleteUser(c *cli.Context) error {
	if !c.IsSet("id") && !c.IsSet("username") && !c.IsSet("email") {
		return errors.New("You must provide the id, username or email of a user to delete")
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	var err error
	var user *user_model.User
	if c.IsSet("email") {
		user, err = user_model.GetUserByEmail(ctx, c.String("email"))
	} else if c.IsSet("username") {
		user, err = user_model.GetUserByName(ctx, c.String("username"))
	} else {
		user, err = user_model.GetUserByID(ctx, c.Int64("id"))
	}
	if err != nil {
		return err
	}
	if c.IsSet("username") && user.LowerName != strings.ToLower(strings.TrimSpace(c.String("username"))) {
		return fmt.Errorf("The user %s who has email %s does not match the provided username %s", user.Name, c.String("email"), c.String("username"))
	}

	if c.IsSet("id") && user.ID != c.Int64("id") {
		return fmt.Errorf("The user %s does not match the provided id %d", user.Name, c.Int64("id"))
	}

	return user_service.DeleteUser(ctx, user, c.Bool("purge"))
}
