// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/urfave/cli/v2"
)

var subcmdTeam = &cli.Command{
	Name:  "team",
	Usage: "Modify teams",
	Subcommands: []*cli.Command{
		microcmdTeamList,
	},
}
