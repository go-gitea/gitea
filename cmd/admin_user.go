// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/urfave/cli/v3"
)

func newUserCommand() *cli.Command {
	return &cli.Command{
		Name:  "user",
		Usage: "Modify users",
		Commands: []*cli.Command{
			microcmdUserCreate(),
			newUserListCommand(),
			microcmdUserChangePassword(),
			microcmdUserDelete(),
			newUserGenerateAccessTokenCommand(),
			microcmdUserMustChangePassword(),
		},
	}
}
