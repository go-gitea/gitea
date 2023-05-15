// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"

	"github.com/urfave/cli"
)

var (
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
			Name:  "stacktrace-level, L",
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
				Name:  "pause",
				Usage: "Pause logging (Gitea will buffer logs up to a certain point and will drop them after that point)",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name: "debug",
					},
				},
				Action: runPauseLogging,
			}, {
				Name:  "resume",
				Usage: "Resume logging",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name: "debug",
					},
				},
				Action: runResumeLogging,
			}, {
				Name:  "release-and-reopen",
				Usage: "Cause Gitea to release and re-open files used for logging",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name: "debug",
					},
				},
				Action: runReleaseReopenLogging,
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
					}, {
						Name:  "conn",
						Usage: "Add a net conn logger",
						Flags: append(defaultLoggingFlags, []cli.Flag{
							cli.BoolFlag{
								Name:  "reconnect-on-message, R",
								Usage: "Reconnect to host for every message",
							}, cli.BoolFlag{
								Name:  "reconnect, r",
								Usage: "Reconnect to host when connection is dropped",
							}, cli.StringFlag{
								Name:  "protocol, P",
								Usage: "Set protocol to use: tcp, unix, or udp (defaults to tcp)",
							}, cli.StringFlag{
								Name:  "address, a",
								Usage: "Host address and port to connect to (defaults to :7020)",
							},
						}...),
						Action: runAddConnLogger,
					}, {
						Name:  "smtp",
						Usage: "Add an SMTP logger",
						Flags: append(defaultLoggingFlags, []cli.Flag{
							cli.StringFlag{
								Name:  "username, u",
								Usage: "Mail server username",
							}, cli.StringFlag{
								Name:  "password, P",
								Usage: "Mail server password",
							}, cli.StringFlag{
								Name:  "host, H",
								Usage: "Mail server host (defaults to: 127.0.0.1:25)",
							}, cli.StringSliceFlag{
								Name:  "send-to, s",
								Usage: "Email address(es) to send to",
							}, cli.StringFlag{
								Name:  "subject, S",
								Usage: "Subject header of sent emails",
							},
						}...),
						Action: runAddSMTPLogger,
					},
				},
			}, {
				Name:  "log-sql",
				Usage: "Set LogSQL",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name: "debug",
					}, cli.BoolFlag{
						Name:  "off",
						Usage: "Switch off SQL logging",
					},
				},
				Action: runSetLogSQL,
			},
		},
	}
)

func runRemoveLogger(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	group := c.String("group")
	if len(group) == 0 {
		group = log.DEFAULT
	}
	name := c.Args().First()

	extra := private.RemoveLogger(ctx, group, name)
	return handleCliResponseExtra(extra)
}

func runAddSMTPLogger(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	vals := map[string]interface{}{}
	mode := "smtp"
	if c.IsSet("host") {
		vals["host"] = c.String("host")
	} else {
		vals["host"] = "127.0.0.1:25"
	}

	if c.IsSet("username") {
		vals["username"] = c.String("username")
	}
	if c.IsSet("password") {
		vals["password"] = c.String("password")
	}

	if !c.IsSet("send-to") {
		return fmt.Errorf("Some recipients must be provided")
	}
	vals["sendTos"] = c.StringSlice("send-to")

	if c.IsSet("subject") {
		vals["subject"] = c.String("subject")
	} else {
		vals["subject"] = "Diagnostic message from Gitea"
	}

	return commonAddLogger(c, mode, vals)
}

func runAddConnLogger(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	vals := map[string]interface{}{}
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
	return commonAddLogger(c, mode, vals)
}

func runAddFileLogger(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
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
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
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
	ctx, cancel := installSignals()
	defer cancel()

	extra := private.AddLogger(ctx, group, name, mode, vals)
	return handleCliResponseExtra(extra)
}

func runPauseLogging(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	userMsg := private.PauseLogging(ctx)
	_, _ = fmt.Fprintln(os.Stdout, userMsg)
	return nil
}

func runResumeLogging(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	userMsg := private.ResumeLogging(ctx)
	_, _ = fmt.Fprintln(os.Stdout, userMsg)
	return nil
}

func runReleaseReopenLogging(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setup(ctx, c.Bool("debug"))
	userMsg := private.ReleaseReopenLogging(ctx)
	_, _ = fmt.Fprintln(os.Stdout, userMsg)
	return nil
}

func runSetLogSQL(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()
	setup(ctx, c.Bool("debug"))

	extra := private.SetLogSQL(ctx, !c.Bool("off"))
	return handleCliResponseExtra(extra)
}
