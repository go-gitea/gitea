// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"

	"github.com/urfave/cli/v3"
)

func newUserCommand() *cli.Command {
	return &cli.Command{
		Name:  "user",
		Usage: "Modify users",
		Before: func(ctx context.Context, _ *cli.Command) (context.Context, error) {
			return cliAuditContext(ctx), nil
		},
		Commands: []*cli.Command{
			microcmdUserCreate(),
			newUserListCommand(),
			microcmdUserChangePassword(),
			microcmdUserDelete(),
			newUserGenerateAccessTokenCommand(),
			microcmdUserMustChangePassword(),
			microcmdUserDisableTwoFactor(),
		},
	}
}
