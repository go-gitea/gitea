// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	cli_docs "github.com/urfave/cli-docs/v3"
	"github.com/urfave/cli/v3"
)

// CmdDocs represents the available docs sub-command.
var CmdDocs = &cli.Command{
	Name:        "docs",
	Usage:       "Output CLI documentation",
	Description: "A command to output Gitea's CLI documentation, optionally to a file.",
	Action:      runDocs,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "man",
			Usage: "Output man pages instead",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Path to output to instead of stdout (will overwrite if exists)",
		},
	},
}

func runDocs(_ context.Context, cmd *cli.Command) error {
	docs, err := cli_docs.ToMarkdown(cmd.Root())
	if cmd.Bool("man") {
		docs, err = cli_docs.ToMan(cmd.Root())
	}
	if err != nil {
		return err
	}

	if !cmd.Bool("man") {
		// Clean up markdown. The following bug was fixed in v2, but is present in v1.
		// It affects markdown output (even though the issue is referring to man pages)
		// https://github.com/urfave/cli/issues/1040
		firstHashtagIndex := strings.Index(docs, "#")

		if firstHashtagIndex > 0 {
			docs = docs[firstHashtagIndex:]
		}
	}

	out := os.Stdout
	if cmd.String("output") != "" {
		fi, err := os.Create(cmd.String("output"))
		if err != nil {
			return err
		}
		defer fi.Close()
		out = fi
	}

	_, err = fmt.Fprintln(out, docs)
	return err
}
