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

	allUsers, err := user_model.GetAllUsers(ctx)
	if err != nil {
		return err
	}

	var users []*user_model.User
	if c.IsSet("admin") {
		for _, u := range allUsers {
			if u.IsAdmin {
				users = append(users, u)
			}
		}
	} else {
		users = allUsers
	}

	if globalBool(c, "output-json") {
		type userInfo struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			IsActive bool   `json:"is_active"`
			IsAdmin  bool   `json:"is_admin"`
			TwoFA    bool   `json:"two_fa,omitempty"`
		}

		userInfos := make([]*userInfo, 0, len(users))
		for _, user := range users {
			userInfos = append(userInfos, &userInfo{
				ID:       user.ID,
				Name:     user.Name,
				Email:    user.Email,
				IsActive: user.IsActive,
				IsAdmin:  user.IsAdmin,
			})
		}

		if c.IsSet("admin") {
			twofa := user_model.UserList(users).GetTwoFaStatus(ctx)
			for _, ui := range userInfos {
				ui.TwoFA = twofa[ui.ID]
			}
		}

		return outputInJSON(userInfos)
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 0, 1, ' ', 0)

	if c.IsSet("admin") {
		fmt.Fprintf(w, "ID\tUsername\tEmail\tIsActive\n")
		for _, u := range users {
			fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", u.ID, u.Name, u.Email, u.IsActive)
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
