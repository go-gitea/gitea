// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	user_model "code.gitea.io/gitea/models/user"

	"github.com/urfave/cli/v2"
)

var microcmdUserList = &cli.Command{
	Name:   "list",
	Usage:  "List users",
	Action: runListUsers,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "admin",
			Usage: "List only admin users",
		},
	},
}

func runListUsers(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	users, err := user_model.GetAllUsers(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 0, 1, ' ', 0)

	if c.IsSet("admin") {
		fmt.Fprintf(w, "ID\tUsername\tEmail\tIsActive\n")
		for _, u := range users {
			if u.IsAdmin {
				fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", u.ID, u.Name, u.Email, u.IsActive)
			}
		}
	} else {
		twofa := user_model.UserList(users).GetTwoFaStatus(ctx)
		fmt.Fprintf(w, "ID\tUsername\tEmail\tIsActive\tIsAdmin\t2FA\n")
		for _, u := range users {
			fmt.Fprintf(w, "%d\t%s\t%s\t%t\t%t\t%t\n", u.ID, u.Name, u.Email, u.IsActive, u.IsAdmin, twofa[u.ID])
		}
	}

	w.Flush()
	return nil
}
