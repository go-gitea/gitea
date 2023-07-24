// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/urfave/cli/v2"
)

var microcmdUserGenerateAccessToken = &cli.Command{
	Name:  "generate-access-token",
	Usage: "Generate an access token for a specific user",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "Username",
		},
		&cli.StringFlag{
			Name:    "token-name",
			Aliases: []string{"t"},
			Usage:   "Token name",
			Value:   "gitea-admin",
		},
		&cli.BoolFlag{
			Name:  "raw",
			Usage: "Display only the token value",
		},
		&cli.StringFlag{
			Name:  "scopes",
			Value: "",
			Usage: "Comma separated list of scopes to apply to access token",
		},
	},
	Action: runGenerateAccessToken,
}

func runGenerateAccessToken(c *cli.Context) error {
	if !c.IsSet("username") {
		return fmt.Errorf("You must provide a username to generate a token for")
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

	accessTokenScope, err := auth_model.AccessTokenScope(c.String("scopes")).Normalize()
	if err != nil {
		return err
	}

	t := &auth_model.AccessToken{
		Name:  c.String("token-name"),
		UID:   user.ID,
		Scope: accessTokenScope,
	}

	if err := auth_model.NewAccessToken(t); err != nil {
		return err
	}

	if c.Bool("raw") {
		fmt.Printf("%s\n", t.Token)
	} else {
		fmt.Printf("Access token was successfully created: %s\n", t.Token)
	}

	return nil
}
