// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"

	user_model "code.gitea.io/gitea/models/user"

	"github.com/urfave/cli/v2"
)

var microcmdUserMustChangePassword = &cli.Command{
	Name:   "must-change-password",
	Usage:  "Set the must change password flag for the provided users or all users",
	Action: runMustChangePassword,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"A"},
			Usage:   "All users must change password, except those explicitly excluded with --exclude",
		},
		&cli.StringSliceFlag{
			Name:    "exclude",
			Aliases: []string{"e"},
			Usage:   "Do not change the must-change-password flag for these users",
		},
		&cli.BoolFlag{
			Name:  "unset",
			Usage: "Instead of setting the must-change-password flag, unset it",
		},
	},
}

func runMustChangePassword(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if c.NArg() == 0 && !c.IsSet("all") {
		return errors.New("either usernames or --all must be provided")
	}

	mustChangePassword := !c.Bool("unset")
	all := c.Bool("all")
	exclude := c.StringSlice("exclude")

	if err := initDB(ctx); err != nil {
		return err
	}

	n, err := user_model.SetMustChangePassword(ctx, all, mustChangePassword, c.Args().Slice(), exclude)
	if err != nil {
		return err
	}

	fmt.Printf("Updated %d users setting MustChangePassword to %t\n", n, mustChangePassword)
	return nil
}
