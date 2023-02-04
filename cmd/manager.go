// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"io"
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
			subcmdLogging,
			subCmdProcesses,
			subCmdCPUProfile,
			subCmdFGProfile,
			subCmdListNamedProfiles,
			subCmdNamedProfile,
			subCmdTrace,
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
			cli.StringFlag{
				Name:  "output,o",
				Usage: "File to output to (set to \"-\" for stdout)",
				Value: "-",
			},
		},
	}
	subCmdCPUProfile = cli.Command{
		Name:   "cpu-profile",
		Usage:  "Return PProf CPU profile",
		Action: runCPUProfile,
		Flags: []cli.Flag{
			cli.DurationFlag{
				Name:  "duration",
				Usage: "Duration to collect CPU Profile over",
				Value: 30 * time.Second,
			},
			cli.StringFlag{
				Name:  "output,o",
				Usage: "File to output to (set to \"-\" for stdout)",
				Value: "cpu-profile",
			},
		},
	}
	subCmdFGProfile = cli.Command{
		Name:   "fg-profile",
		Usage:  "Return PProf Full Go profile",
		Action: runFGProfile,
		Flags: []cli.Flag{
			cli.DurationFlag{
				Name:  "duration",
				Usage: "Duration to collect CPU Profile over",
				Value: 30 * time.Second,
			},
			cli.StringFlag{
				Name:  "format",
				Usage: "Format to return the profile in: pprof, folded",
				Value: "pprof",
			},
			cli.StringFlag{
				Name:  "output,o",
				Usage: "File to output to (set to \"-\" for stdout)",
				Value: "fg-profile",
			},
		},
	}
	subCmdNamedProfile = cli.Command{
		Name:   "named-profile",
		Usage:  "Return PProf named profile",
		Action: runNamedProfile,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name",
				Usage: "Name of profile to run",
			},
			cli.IntFlag{
				Name:  "debug-level",
				Usage: "Debug level for the profile",
			},
			cli.StringFlag{
				Name:  "output,o",
				Usage: "File to output to (set to \"-\" for stdout)",
			},
		},
	}
	subCmdListNamedProfiles = cli.Command{
		Name:   "list-named-profiles",
		Usage:  "Return PProf list of named profiles",
		Action: runListNamedProfile,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "json",
				Usage: "Output as json",
			},
			cli.StringFlag{
				Name:  "output,o",
				Usage: "File to output to (set to \"-\" for stdout)",
				Value: "-",
			},
		},
	}
	subCmdTrace = cli.Command{
		Name:   "trace",
		Usage:  "Return PProf trace",
		Action: runTrace,
		Flags: []cli.Flag{
			cli.DurationFlag{
				Name:  "duration",
				Usage: "Duration to collect CPU Profile over",
				Value: 30 * time.Second,
			},
			cli.StringFlag{
				Name:  "output,o",
				Usage: "File to output to (set to \"-\" for stdout)",
				Value: "trace",
			},
		},
	}
)

func runShutdown(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup("manager", c.Bool("debug"))
	statusCode, msg := private.Shutdown(ctx)
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}

func runRestart(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup("manager", c.Bool("debug"))
	statusCode, msg := private.Restart(ctx)
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}

func runFlushQueues(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup("manager", c.Bool("debug"))
	statusCode, msg := private.FlushQueues(ctx, c.Duration("timeout"), c.Bool("non-blocking"))
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}

func determineOutput(c *cli.Context, defaultFilename string) (io.WriteCloser, error) {
	out := os.Stdout
	filename := c.String("output")
	if filename == "" {
		filename = defaultFilename
	}
	if filename != "-" {
		var err error
		out, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return nil, fail("Unable to open "+filename, err.Error())
		}
		fmt.Printf("Writing to %s\n", filename)
	}
	return out, nil
}

func runProcesses(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup("manager", c.Bool("debug"))
	out, err := determineOutput(c, "-")
	if err != nil {
		return err
	}
	defer out.Close()

	statusCode, msg := private.Processes(ctx, out, c.Bool("flat"), c.Bool("no-system"), c.Bool("stacktraces"), c.Bool("json"), c.String("cancel"))
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}

	return nil
}

func runCPUProfile(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()
	setup("manager", c.Bool("debug"))

	out, err := determineOutput(c, "cpu-profile")
	if err != nil {
		return err
	}
	defer out.Close()

	statusCode, msg := private.CPUProfile(ctx, out, c.Duration("duration"))
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}
	return nil
}

func runFGProfile(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()
	setup("manager", c.Bool("debug"))
	out, err := determineOutput(c, "fg-profile")
	if err != nil {
		return err
	}
	defer out.Close()

	statusCode, msg := private.FGProfile(ctx, out, c.Duration("duration"), c.String("format"))
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}
	return nil
}

func runNamedProfile(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()
	setup("manager", c.Bool("debug"))
	out, err := determineOutput(c, c.String("name")+"-profile")
	if err != nil {
		return err
	}
	defer out.Close()
	statusCode, msg := private.NamedProfile(ctx, out, c.String("name"), c.Int("debug-level"))
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}
	return nil
}

func runListNamedProfile(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()
	setup("manager", c.Bool("debug"))

	out, err := determineOutput(c, "-")
	if err != nil {
		return err
	}
	defer out.Close()

	statusCode, msg := private.ListNamedProfiles(ctx, out, c.Bool("json"))
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}
	return nil
}

func runTrace(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()
	setup("manager", c.Bool("debug"))

	out, err := determineOutput(c, "trace")
	if err != nil {
		return err
	}
	defer out.Close()

	statusCode, msg := private.Trace(ctx, out, c.Duration("duration"))
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}
	return nil
}
