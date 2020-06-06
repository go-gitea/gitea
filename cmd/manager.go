// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"code.gitea.io/gitea/modules/log"
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
			}, cli.BoolFlag{
				Name:  "non-blocking",
				Usage: "Set to true to not wait for flush to complete before returning",
			},
		},
	}
	defaultLoggingFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "group, g",
			Usage: "Group to add logger to - will default to \"default\"",
		}, cli.StringFlag{
			Name:  "name, n",
			Usage: "Name of the new logger - will default to mode",
		}, cli.StringFlag{
			Name:  "level, l",
			Usage: "Logging level for the new logger",
		}, cli.StringFlag{
			Name:  "stacktrace-level, S",
			Usage: "Stacktrace logging level",
		}, cli.StringFlag{
			Name:  "flags, F",
			Usage: "Flags for the logger",
		}, cli.StringFlag{
			Name:  "expression, e",
			Usage: "Matching expression for the logger",
		}, cli.StringFlag{
			Name:  "prefix, p",
			Usage: "Prefix for the logger",
		}, cli.BoolFlag{
			Name:  "color",
			Usage: "Use color in the logs",
		}, cli.BoolFlag{
			Name: "debug",
		},
	}
	subcmdLogging = cli.Command{
		Name:  "logging",
		Usage: "Adjust logging commands",
		Subcommands: []cli.Command{
			{
				Name:   "pause",
				Usage:  "Pause logging (Gitea will buffer logs up to a certain point and will drop them after that point)",
				Action: runPauseLogging,
			}, {
				Name:   "resume",
				Usage:  "Resume logging",
				Action: runResumeLogging,
			}, {
				Name:      "remove",
				Usage:     "Remove a logger",
				ArgsUsage: "[name] Name of logger to remove",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name: "debug",
					}, cli.StringFlag{
						Name:  "group, g",
						Usage: "Group to add logger to - will default to \"default\"",
					},
				},
				Action: runRemoveLogger,
			}, {
				Name:  "add",
				Usage: "Add a logger",
				Subcommands: []cli.Command{
					{
						Name:  "console",
						Usage: "Add a console logger",
						Flags: append(defaultLoggingFlags,
							cli.BoolFlag{
								Name:  "stderr",
								Usage: "Output console logs to stderr - only relevant for console",
							}),
						Action: runAddConsoleLogger,
					}, {
						Name:  "file",
						Usage: "Add a file logger",
						Flags: append(defaultLoggingFlags, []cli.Flag{
							cli.StringFlag{
								Name:  "filename, f",
								Usage: "Filename for the logger - this must be set.",
							}, cli.BoolTFlag{
								Name:  "rotate, r",
								Usage: "Rotate logs",
							}, cli.Int64Flag{
								Name:  "max-size, s",
								Usage: "Maximum size in bytes before rotation",
							}, cli.BoolTFlag{
								Name:  "daily, d",
								Usage: "Rotate logs daily",
							}, cli.IntFlag{
								Name:  "max-days, D",
								Usage: "Maximum number of daily logs to keep",
							}, cli.BoolTFlag{
								Name:  "compress, z",
								Usage: "Compress rotated logs",
							}, cli.IntFlag{
								Name:  "compression-level, Z",
								Usage: "Compression level to use",
							},
						}...),
						Action: runAddFileLogger,
					},
				},
			},
		},
	}
)

func runRemoveLogger(c *cli.Context) error {
	setup("manager", c.Bool("debug"))
	group := c.String("group")
	if len(group) == 0 {
		group = log.DEFAULT
	}
	name := c.Args().First()
	statusCode, msg := private.RemoveLogger(group, name)
	switch statusCode {
	case http.StatusInternalServerError:
		fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}

func runAddFileLogger(c *cli.Context) error {
	setup("manager", c.Bool("debug"))
	vals := map[string]interface{}{}
	mode := "file"
	if c.IsSet("filename") {
		vals["filename"] = c.String("filename")
	} else {
		return fmt.Errorf("filename must be set when creating a file logger")
	}
	if c.IsSet("rotate") {
		vals["rotate"] = c.Bool("rotate")
	}
	if c.IsSet("max-size") {
		vals["maxsize"] = c.Int64("max-size")
	}
	if c.IsSet("daily") {
		vals["daily"] = c.Bool("daily")
	}
	if c.IsSet("max-days") {
		vals["maxdays"] = c.Int("max-days")
	}
	if c.IsSet("compress") {
		vals["compress"] = c.Bool("compress")
	}
	if c.IsSet("compression-level") {
		vals["compressionLevel"] = c.Int("compression-level")
	}
	return commonAddLogger(c, mode, vals)
}

func runAddConsoleLogger(c *cli.Context) error {
	setup("manager", c.Bool("debug"))
	vals := map[string]interface{}{}
	mode := "console"
	if c.IsSet("stderr") && c.Bool("stderr") {
		vals["stderr"] = c.Bool("stderr")
	}
	return commonAddLogger(c, mode, vals)
}

func commonAddLogger(c *cli.Context, mode string, vals map[string]interface{}) error {
	if len(c.String("level")) > 0 {
		vals["level"] = log.FromString(c.String("level")).String()
	}
	if len(c.String("stacktrace-level")) > 0 {
		vals["stacktraceLevel"] = log.FromString(c.String("stacktrace-level")).String()
	}
	if len(c.String("expression")) > 0 {
		vals["expression"] = c.String("expression")
	}
	if len(c.String("prefix")) > 0 {
		vals["prefix"] = c.String("prefix")
	}
	if len(c.String("flags")) > 0 {
		vals["flags"] = log.FlagsFromString(c.String("flags"))
	}
	if c.IsSet("color") {
		vals["colorize"] = c.Bool("color")
	}
	group := "default"
	if c.IsSet("group") {
		group = c.String("group")
	}
	name := mode
	if c.IsSet("name") {
		name = c.String("name")
	}
	statusCode, msg := private.AddLogger(group, name, mode, vals)
	switch statusCode {
	case http.StatusInternalServerError:
		fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}

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

func runPauseLogging(c *cli.Context) error {
	setup("manager", false)
	statusCode, msg := private.PauseLogging()
	switch statusCode {
	case http.StatusInternalServerError:
		fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}

func runResumeLogging(c *cli.Context) error {
	setup("manager", false)
	statusCode, msg := private.ResumeLogging()
	switch statusCode {
	case http.StatusInternalServerError:
		fail("InternalServerError", msg)
	}

	fmt.Fprintln(os.Stdout, msg)
	return nil
}
