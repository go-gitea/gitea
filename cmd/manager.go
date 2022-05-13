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

	"github.com/urfave/cli/v2"
)

var (
	// CmdManager represents the manager command
	CmdManager = cli.Command{
		Name:        "manager",
		Usage:       "Manage the running gitea process",
		Description: "This is a command for managing the running gitea process",
		Subcommands: []*cli.Command{
			&subcmdShutdown,
			&subcmdRestart,
			&subcmdFlushQueues,
			&subcmdLogging,
			&subCmdProcesses,
		},
	}
	subcmdShutdown = cli.Command{
		Name:  "shutdown",
		Usage: "Gracefully shutdown the running process",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "debug",
			},
		},
		Action: runShutdown,
	}
	subcmdRestart = cli.Command{
		Name:  "restart",
		Usage: "Gracefully restart the running process - (not implemented for windows servers)",
		Flags: []cli.Flag{
			&cli.BoolFlag{
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
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 60 * time.Second,
				Usage: "Timeout for the flushing process",
			},
			&cli.BoolFlag{
				Name:  "non-blocking",
				Usage: "Set to true to not wait for flush to complete before returning",
			},
			&cli.BoolFlag{
				Name: "debug",
			},
		},
	}
<<<<<<< HEAD
	defaultLoggingFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    "group",
			Aliases: []string{"g"},
			Usage:   "Group to add logger to - will default to \"default\"",
		},
		&cli.StringFlag{
			Name:    "name",
			Aliases: []string{"n"},
			Usage:   "Name of the new logger - will default to mode",
		},
		&cli.StringFlag{
			Name:    "level",
			Aliases: []string{"l"},
			Usage:   "Logging level for the new logger",
		},
		&cli.StringFlag{
			Name:    "stacktrace-level",
			Aliases: []string{"L"},
			Usage:   "Stacktrace logging level",
		},
		&cli.StringFlag{
			Name:    "flags",
			Aliases: []string{"F"},
			Usage:   "Flags for the logger",
		},
		&cli.StringFlag{
			Name:    "expression",
			Aliases: []string{"e"},
			Usage:   "Matching expression for the logger",
		},
		&cli.StringFlag{
			Name:    "prefix",
			Aliases: []string{"p"},
			Usage:   "Prefix for the logger",
		},
		&cli.BoolFlag{
			Name:  "color",
			Usage: "Use color in the logs",
		},
		&cli.BoolFlag{
			Name: "debug",
		},
	}
	subcmdLogging = cli.Command{
		Name:  "logging",
		Usage: "Adjust logging commands",
		Subcommands: []*cli.Command{
			{
				Name:  "pause",
				Usage: "Pause logging (Gitea will buffer logs up to a certain point and will drop them after that point)",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name: "debug",
					},
				},
				Action: runPauseLogging,
			}, {
				Name:  "resume",
				Usage: "Resume logging",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name: "debug",
					},
				},
				Action: runResumeLogging,
			}, {
				Name:  "release-and-reopen",
				Usage: "Cause Gitea to release and re-open files used for logging",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name: "debug",
					},
				},
				Action: runReleaseReopenLogging,
			}, {
				Name:      "remove",
				Usage:     "Remove a logger",
				ArgsUsage: "[name] Name of logger to remove",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name: "debug",
					},
					&cli.StringFlag{
						Name:    "group",
						Aliases: []string{"g"},
						Usage:   "Group to add logger to - will default to \"default\"",
					},
				},
				Action: runRemoveLogger,
			}, {
				Name:  "add",
				Usage: "Add a logger",
				Subcommands: []*cli.Command{
					{
						Name:  "console",
						Usage: "Add a console logger",
						Flags: append(defaultLoggingFlags,
							&cli.BoolFlag{
								Name:  "stderr",
								Usage: "Output console logs to stderr - only relevant for console",
							}),
						Action: runAddConsoleLogger,
					}, {
						Name:  "file",
						Usage: "Add a file logger",
						Flags: append(defaultLoggingFlags, []cli.Flag{
							&cli.StringFlag{
								Name:    "filename",
								Aliases: []string{"f"},
								Usage:   "Filename for the logger - this must be set.",
							},
							&cli.BoolFlag{
								Name:    "rotate",
								Aliases: []string{"r"},
								Usage:   "Rotate logs",
							},
							&cli.Int64Flag{
								Name:    "max-size",
								Aliases: []string{"s"},
								Usage:   "Maximum size in bytes before rotation",
							},
							&cli.BoolFlag{
								Name:    "daily",
								Aliases: []string{"d"},
								Usage:   "Rotate logs daily",
							},
							&cli.IntFlag{
								Name:    "max-days",
								Aliases: []string{"D"},
								Usage:   "Maximum number of daily logs to keep",
							},
							&cli.BoolFlag{
								Name:    "compress",
								Aliases: []string{"z"},
								Usage:   "Compress rotated logs",
							},
							&cli.IntFlag{
								Name:    "compression-level",
								Aliases: []string{"Z"},
								Usage:   "Compression level to use",
							},
						}...),
						Action: runAddFileLogger,
					}, {
						Name:  "conn",
						Usage: "Add a net conn logger",
						Flags: append(defaultLoggingFlags, []cli.Flag{
							&cli.BoolFlag{
								Name:    "reconnect-on-message",
								Aliases: []string{"R"},
								Usage:   "Reconnect to host for every message",
							},
							&cli.BoolFlag{
								Name:    "reconnect",
								Aliases: []string{"r"},
								Usage:   "Reconnect to host when connection is dropped",
							},
							&cli.StringFlag{
								Name:    "protocol",
								Aliases: []string{"P"},
								Usage:   "Set protocol to use: tcp, unix, or udp (defaults to tcp)",
							},
							&cli.StringFlag{
								Name:    "address",
								Aliases: []string{"a"},
								Usage:   "Host address and port to connect to (defaults to :7020)",
							},
						}...),
						Action: runAddConnLogger,
					}, {
						Name:  "smtp",
						Usage: "Add an SMTP logger",
						Flags: append(defaultLoggingFlags, []cli.Flag{
							&cli.StringFlag{
								Name:    "username",
								Aliases: []string{"u"},
								Usage:   "Mail server username",
							},
							&cli.StringFlag{
								Name:    "password",
								Aliases: []string{"P"},
								Usage:   "Mail server password",
							},
							&cli.StringFlag{
								Name:    "host",
								Aliases: []string{"H"},
								Usage:   "Mail server host (defaults to: 127.0.0.1:25)",
							},
							&cli.StringSliceFlag{
								Name:    "send-to",
								Aliases: []string{"s"},
								Usage:   "Email address(es) to send to",
							},
							&cli.StringFlag{
								Name:    "subject",
								Aliases: []string{"S"},
								Usage:   "Subject header of sent emails",
							},
						}...),
						Action: runAddSMTPLogger,
					},
				},
=======
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
				Usage: "Do not show system proceses",
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
>>>>>>> master
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

func runProcesses(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup("manager", c.Bool("debug"))
	statusCode, msg := private.Processes(ctx, os.Stdout, c.Bool("flat"), c.Bool("no-system"), c.Bool("stacktraces"), c.Bool("json"), c.String("cancel"))
	switch statusCode {
	case http.StatusInternalServerError:
		return fail("InternalServerError", msg)
	}

	return nil
}
