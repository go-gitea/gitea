// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"strings"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"

	"github.com/urfave/cli/v3"
)

// parseUserTypeFlag parses "--user-type", keeping the lowercase "individual" and "bot" the flag shipped with
func parseUserTypeFlag(s string) (user_model.UserType, error) {
	switch strings.ToLower(s) {
	case "user", "individual":
		return user_model.UserTypeIndividual, nil
	case "bot":
		return user_model.UserTypeBot, nil
	default:
		return 0, util.NewInvalidArgumentErrorf("invalid user type %q (expected User, Bot)", s)
	}
}

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
			microcmdUserChangeType(),
		},
	}
}
