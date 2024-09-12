// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/urfave/cli/v2"
)

var subcmdUser = &cli.Command{
	Name:  "user",
	Usage: "Modify users",
	Subcommands: []*cli.Command{
		microcmdUserCreate,
		microcmdUserList,
		microcmdUserChangePassword,
		microcmdUserDelete,
		microcmdUserGenerateAccessToken,
		microcmdUserMustChangePassword,
	},
}
