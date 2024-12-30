// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
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
		return errors.New("You must provide a username to generate a token for")
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

	// construct token with name and user so we can make sure it is unique
	t := &auth_model.AccessToken{
		Name: c.String("token-name"),
		UID:  user.ID,
	}

	exist, err := auth_model.AccessTokenByNameExists(ctx, t)
	if err != nil {
		return err
	}
	if exist {
		return errors.New("access token name has been used already")
	}

	// make sure the scopes are valid
	accessTokenScope, err := auth_model.AccessTokenScope(c.String("scopes")).Normalize()
	if err != nil {
		return fmt.Errorf("invalid access token scope provided: %w", err)
	}
	t.Scope = accessTokenScope

	// create the token
	if err := auth_model.NewAccessToken(ctx, t); err != nil {
		return err
	}

	if c.Bool("raw") {
		fmt.Printf("%s\n", t.Token)
	} else {
		fmt.Printf("Access token was successfully created: %s\n", t.Token)
	}

	return nil
}
