// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"time"

	"code.gitea.io/gitea/modules/private"

	"github.com/urfave/cli"
)

var (
	// CmdManager represents the manager command
	CmdManager = cli.Command{
		Name:        "manager",
		Usage:       "Manage the running gitea process",
		Description: "This is a command for managing the running gitea process",
		Subcommands: []cli.Command{
			subcmdShutdown,
			subcmdRestart,
			subcmdReloadTemplates,
			subcmdFlushQueues,
			subcmdLogging,
			subCmdProcesses,
		},
	}
	subcmdShutdown = cli.Command{
		Name:  "shutdown",
		Usage: "Gracefully shutdown the running process",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
		},
		Action: runShutdown,
	}
	subcmdRestart = cli.Command{
		Name:  "restart",
		Usage: "Gracefully restart the running process - (not implemented for windows servers)",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
		},
		Action: runRestart,
	}
	subcmdReloadTemplates = cli.Command{
		Name:  "reload-templates",
		Usage: "Reload template files in the running process",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
		},
		Action: runReloadTemplates,
	}
	subcmdFlushQueues = cli.Command{
		Name:   "flush-queues",
		Usage:  "Flush queues in the running process",
		Action: runFlushQueues,
		Flags: []cli.Flag{
			cli.DurationFlag{
				Name:  "timeout",
				Value: 60 * time.Second,
				Usage: "Timeout for the flushing process",
			},
			cli.BoolFlag{
				Name:  "non-blocking",
				Usage: "Set to true to not wait for flush to complete before returning",
			},
			cli.BoolFlag{
				Name: "debug",
			},
		},
	}
	subCmdProcesses = cli.Command{
		Name:   "processes",
		Usage:  "Display running processes within the current process",
		Action: runProcesses,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
			cli.BoolFlag{
				Name:  "flat",
				Usage: "Show processes as flat table rather than as tree",
			},
			cli.BoolFlag{
				Name:  "no-system",
				Usage: "Do not show system processes",
			},
			cli.BoolFlag{
				Name:  "stacktraces",
				Usage: "Show stacktraces",
			},
			cli.BoolFlag{
				Name:  "json",
				Usage: "Output as json",
			},
			cli.StringFlag{
				Name:  "cancel",
				Usage: "Process PID to cancel. (Only available for non-system processes.)",
			},
		},
	}
)

func runShutdown(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	extra := private.Shutdown(ctx)
	return handleCliResponseExtra(extra)
}

func runRestart(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	extra := private.Restart(ctx)
	return handleCliResponseExtra(extra)
}

func runReloadTemplates(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	extra := private.ReloadTemplates(ctx)
	return handleCliResponseExtra(extra)
}

func runFlushQueues(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	extra := private.FlushQueues(ctx, c.Duration("timeout"), c.Bool("non-blocking"))
	return handleCliResponseExtra(extra)
}

func runProcesses(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	extra := private.Processes(ctx, os.Stdout, c.Bool("flat"), c.Bool("no-system"), c.Bool("stacktraces"), c.Bool("json"), c.String("cancel"))
	return handleCliResponseExtra(extra)
}
