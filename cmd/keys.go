// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"

	"github.com/urfave/cli/v2"
)

// CmdKeys represents the available keys sub-command
var CmdKeys = &cli.Command{
	Name:   "keys",
	Usage:  "This command queries the Gitea database to get the authorized command for a given ssh key fingerprint",
	Before: PrepareConsoleLoggerLevel(log.FATAL),
	Action: runKeys,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "expected",
			Aliases: []string{"e"},
			Value:   "git",
			Usage:   "Expected user for whom provide key commands",
		},
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Value:   "",
			Usage:   "Username trying to log in by SSH",
		},
		&cli.StringFlag{
			Name:    "type",
			Aliases: []string{"t"},
			Value:   "",
			Usage:   "Type of the SSH key provided to the SSH Server (requires content to be provided too)",
		},
		&cli.StringFlag{
			Name:    "content",
			Aliases: []string{"k"},
			Value:   "",
			Usage:   "Base64 encoded content of the SSH key provided to the SSH Server (requires type to be provided too)",
		},
	},
}

func runKeys(c *cli.Context) error {
	if !c.IsSet("username") {
		return errors.New("No username provided")
	}
	// Check username matches the expected username
	if strings.TrimSpace(c.String("username")) != strings.TrimSpace(c.String("expected")) {
		return nil
	}

	content := ""

	if c.IsSet("type") && c.IsSet("content") {
		content = fmt.Sprintf("%s %s", strings.TrimSpace(c.String("type")), strings.TrimSpace(c.String("content")))
	}

	if content == "" {
		return errors.New("No key type and content provided")
	}

	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, false)

	authorizedString, extra := private.AuthorizedPublicKeyByContent(ctx, content)
	// do not use handleCliResponseExtra or cli.NewExitError, if it exists immediately, it breaks some tests like Test_CmdKeys
	if extra.Error != nil {
		return extra.Error
	}
	_, _ = fmt.Fprintln(c.App.Writer, strings.TrimSpace(authorizedString))
	return nil
}
