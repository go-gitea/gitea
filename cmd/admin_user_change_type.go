// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	user_service "gitea.dev/services/user"

	"github.com/urfave/cli/v3"
)

func microcmdUserChangeType() *cli.Command {
	return &cli.Command{
		Name:   "change-type",
		Usage:  "Convert a user between the individual and bot types",
		Action: runChangeUserType,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "username",
				Aliases:  []string{"u"},
				Usage:    "The user to convert",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "user-type",
				Usage:    "New user type: individual or bot",
				Required: true,
			},
		},
	}
}

func runChangeUserType(ctx context.Context, c *cli.Command) error {
	targetType, err := parseUserType(c.String("user-type"))
	if err != nil {
		return err
	}

	if !setting.IsInTesting {
		if err := initDB(ctx); err != nil {
			return err
		}
	}

	user, err := user_model.GetUserByName(ctx, c.String("username"))
	if err != nil {
		return err
	}

	if err := user_service.ConvertUserType(ctx, user, targetType); err != nil {
		return err
	}

	fmt.Printf("%s's type has been successfully changed to %s!\n", user.Name, c.String("user-type"))
	return nil
}

// parseUserType maps a CLI user-type string to the convertible user types.
func parseUserType(s string) (user_model.UserType, error) {
	switch s {
	case "individual":
		return user_model.UserTypeIndividual, nil
	case "bot":
		return user_model.UserTypeBot, nil
	default:
		return 0, fmt.Errorf("invalid user type %q, must be one of: individual, bot", s)
	}
}
