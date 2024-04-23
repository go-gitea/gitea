// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	auth_service "code.gitea.io/gitea/services/auth"

	"github.com/urfave/cli/v2"
)

var (
	microcmdAuthDelete = &cli.Command{
		Name:   "delete",
		Usage:  "Delete specific auth source",
		Flags:  []cli.Flag{idFlag},
		Action: runDeleteAuth,
	}
	microcmdAuthList = &cli.Command{
		Name:   "list",
		Usage:  "List auth sources",
		Action: runListAuth,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "min-width",
				Usage: "Minimal cell width including any padding for the formatted table",
				Value: 0,
			},
			&cli.IntFlag{
				Name:  "tab-width",
				Usage: "width of tab characters in formatted table (equivalent number of spaces)",
				Value: 8,
			},
			&cli.IntFlag{
				Name:  "padding",
				Usage: "padding added to a cell before computing its width",
				Value: 1,
			},
			&cli.StringFlag{
				Name:  "pad-char",
				Usage: `ASCII char used for padding if padchar == '\\t', the Writer will assume that the width of a '\\t' in the formatted output is tabwidth, and cells are left-aligned independent of align_left (for correct-looking results, tabwidth must correspond to the tab width in the viewer displaying the result)`,
				Value: "\t",
			},
			&cli.BoolFlag{
				Name:  "vertical-bars",
				Usage: "Set to true to print vertical bars between columns",
			},
		},
	}
)

func runListAuth(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	authSources, err := db.Find[auth_model.Source](ctx, auth_model.FindSourcesOptions{})
	if err != nil {
		return err
	}

	flags := tabwriter.AlignRight
	if c.Bool("vertical-bars") {
		flags |= tabwriter.Debug
	}

	padChar := byte('\t')
	if len(c.String("pad-char")) > 0 {
		padChar = c.String("pad-char")[0]
	}

	// loop through each source and print
	w := tabwriter.NewWriter(os.Stdout, c.Int("min-width"), c.Int("tab-width"), c.Int("padding"), padChar, flags)
	fmt.Fprintf(w, "ID\tName\tType\tEnabled\n")
	for _, source := range authSources {
		fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", source.ID, source.Name, source.Type.String(), source.IsActive)
	}
	w.Flush()

	return nil
}

func runDeleteAuth(c *cli.Context) error {
	if !c.IsSet("id") {
		return errors.New("--id flag is missing")
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	source, err := auth_model.GetSourceByID(ctx, c.Int64("id"))
	if err != nil {
		return err
	}

	return auth_service.DeleteSource(ctx, source)
}
