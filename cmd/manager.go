// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"net/http"
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
			subcmdFlushQueues,
		},
	}
	subcmdShutdown = cli.Command{
		Name:   "shutdown",
		Usage:  "Gracefully shutdown the running process",
		Action: runShutdown,
	}
	subcmdRestart = cli.Command{
		Name:   "restart",
		Usage:  "Gracefully restart the running process - (not implemented for windows servers)",
		Action: runRestart,
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
		},
	}
)

func runShutdown(c *cli.Context) error {
	setup("manager", false)
	statusCode, msg := private.Shutdown()
	switch statusCode {
	case http.StatusInternalServerError:
		fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}

func runRestart(c *cli.Context) error {
	setup("manager", false)
	statusCode, msg := private.Restart()
	switch statusCode {
	case http.StatusInternalServerError:
		fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}

func runFlushQueues(c *cli.Context) error {
	setup("manager", false)
	statusCode, msg := private.FlushQueues(c.Duration("timeout"), c.Bool("non-blocking"))
	switch statusCode {
	case http.StatusInternalServerError:
		fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}
