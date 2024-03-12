// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"code.gitea.io/gitea/models/db"
	team_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"

	"github.com/urfave/cli/v2"
)

var microcmdTeamList = &cli.Command{
	Name:   "list",
	Usage:  "List teams",
	Action: runListTeams,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "owner",
			Usage: "List owner teams",
		},
	},
}

func runListTeams(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	teams := make([]*team_model.Team, 0)
	err := db.GetEngine(ctx).Table("team").Find(&teams)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 0, 1, ' ', 0)

	fmt.Fprintf(w, "ID\tName\tAccessMode\n")
	for _, t := range teams {
		if t.AccessMode != perm.AccessModeOwner || c.IsSet("owner") {
			fmt.Fprintf(w, "%d\t%s\t%s\n", t.ID, t.Name, t.AccessMode)
		}
	}

	w.Flush()
	return nil
}
