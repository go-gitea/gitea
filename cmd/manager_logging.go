// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"

	"github.com/urfave/cli/v3"
)

var (
	defaultLoggingFlags = []cli.Flag{
		&cli.StringFlag{
			Name:  "logger",
			Usage: `Logger name - will default to "default"`,
		},
		&cli.StringFlag{
			Name:  "writer",
			Usage: "Name of the log writer - will default to mode",
		},
		&cli.StringFlag{
			Name:  "level",
			Usage: "Logging level for the new logger",
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

	subcmdLogging = &cli.Command{
		Name:  "logging",
		Usage: "Adjust logging commands",
		Commands: []*cli.Command{
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
					}, &cli.StringFlag{
						Name:  "logger",
						Usage: `Logger name - will default to "default"`,
					},
				},
				Action: runRemoveLogger,
			}, {
				Name:  "add",
				Usage: "Add a logger",
				Commands: []*cli.Command{
					{
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
								Value:   true,
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
								Value:   true,
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
								Value:   true,
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
					},
				},
			}, {
				Name:  "log-sql",
				Usage: "Set LogSQL",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name: "debug",
					},
					&cli.BoolFlag{
						Name:  "off",
						Usage: "Switch off SQL logging",
					},
				},
				Action: runSetLogSQL,
			},
		},
	}
)

func runRemoveLogger(ctx context.Context, c *cli.Command) error {
	setup(ctx, c.Bool("debug"))
	logger := c.String("logger")
	if len(logger) == 0 {
		logger = log.DEFAULT
	}
	writer := c.Args().First()

	extra := private.RemoveLogger(ctx, logger, writer)
	return handleCliResponseExtra(extra)
}

func runAddConnLogger(ctx context.Context, c *cli.Command) error {
	setup(ctx, c.Bool("debug"))
	vals := map[string]any{}
	mode := "conn"
	vals["net"] = "tcp"
	if c.IsSet("protocol") {
		switch c.String("protocol") {
		case "udp":
			vals["net"] = "udp"
		case "unix":
			vals["net"] = "unix"
		}
	}
	if c.IsSet("address") {
		vals["address"] = c.String("address")
	} else {
		vals["address"] = ":7020"
	}
	if c.IsSet("reconnect") {
		vals["reconnect"] = c.Bool("reconnect")
	}
	if c.IsSet("reconnect-on-message") {
		vals["reconnectOnMsg"] = c.Bool("reconnect-on-message")
	}
	return commonAddLogger(ctx, c, mode, vals)
}

func runAddFileLogger(ctx context.Context, c *cli.Command) error {
	setup(ctx, c.Bool("debug"))
	vals := map[string]any{}
	mode := "file"
	if c.IsSet("filename") {
		vals["filename"] = c.String("filename")
	} else {
		return errors.New("filename must be set when creating a file logger")
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
	return commonAddLogger(ctx, c, mode, vals)
}

func commonAddLogger(ctx context.Context, c *cli.Command, mode string, vals map[string]any) error {
	if len(c.String("level")) > 0 {
		vals["level"] = log.LevelFromString(c.String("level")).String()
	}
	if len(c.String("stacktrace-level")) > 0 {
		vals["stacktraceLevel"] = log.LevelFromString(c.String("stacktrace-level")).String()
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
	logger := log.DEFAULT
	if c.IsSet("logger") {
		logger = c.String("logger")
	}
	writer := mode
	if c.IsSet("writer") {
		writer = c.String("writer")
	}

	extra := private.AddLogger(ctx, logger, writer, mode, vals)
	return handleCliResponseExtra(extra)
}

func runPauseLogging(ctx context.Context, c *cli.Command) error {
	setup(ctx, c.Bool("debug"))
	userMsg := private.PauseLogging(ctx)
	_, _ = fmt.Fprintln(os.Stdout, userMsg)
	return nil
}

func runResumeLogging(ctx context.Context, c *cli.Command) error {
	setup(ctx, c.Bool("debug"))
	userMsg := private.ResumeLogging(ctx)
	_, _ = fmt.Fprintln(os.Stdout, userMsg)
	return nil
}

func runReleaseReopenLogging(ctx context.Context, c *cli.Command) error {
	setup(ctx, c.Bool("debug"))
	userMsg := private.ReleaseReopenLogging(ctx)
	_, _ = fmt.Fprintln(os.Stdout, userMsg)
	return nil
}

func runSetLogSQL(ctx context.Context, c *cli.Command) error {
	setup(ctx, c.Bool("debug"))

	extra := private.SetLogSQL(ctx, !c.Bool("off"))
	return handleCliResponseExtra(extra)
}
