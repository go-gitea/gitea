// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	golog "log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/migrations"
	migrate_base "code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/doctor"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v2"
	"xorm.io/xorm"
)

var cmdDoctorCheck = &cli.Command{
	Name:        "check",
	Usage:       "Diagnose and optionally fix problems",
	Description: "A command to diagnose problems with the current Gitea instance according to the given configuration. Some problems can optionally be fixed by modifying the database or data storage.",
	Action:      runDoctorCheck,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "list",
			Usage: "List the available checks",
		},
		&cli.BoolFlag{
			Name:  "default",
			Usage: "Run the default checks (if neither --run or --all is set, this is the default behaviour)",
		},
		&cli.StringSliceFlag{
			Name:  "run",
			Usage: "Run the provided checks - (if --default is set, the default checks will also run)",
		},
		&cli.BoolFlag{
			Name:  "all",
			Usage: "Run all the available checks",
		},
		&cli.BoolFlag{
			Name:  "fix",
			Usage: "Automatically fix what we can",
		},
		&cli.StringFlag{
			Name:  "log-file",
			Usage: `Name of the log file (no verbose log output by default). Set to "-" to output to stdout`,
		},
		&cli.BoolFlag{
			Name:    "color",
			Aliases: []string{"H"},
			Usage:   "Use color for outputted information",
		},
	},
}

// CmdDoctor represents the available doctor sub-command.
var CmdDoctor = &cli.Command{
	Name:        "doctor",
	Usage:       "Diagnose and optionally fix problems",
	Description: "A command to diagnose problems with the current Gitea instance according to the given configuration. Some problems can optionally be fixed by modifying the database or data storage.",

	Subcommands: []*cli.Command{
		cmdDoctorCheck,
		cmdRecreateTable,
		cmdDoctorConvert,
	},
}

var cmdRecreateTable = &cli.Command{
	Name:      "recreate-table",
	Usage:     "Recreate tables from XORM definitions and copy the data.",
	ArgsUsage: "[TABLE]... : (TABLEs to recreate - leave blank for all)",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "Print SQL commands sent",
		},
	},
	Description: `The database definitions Gitea uses change across versions, sometimes changing default values and leaving old unused columns.

This command will cause Xorm to recreate tables, copying over the data and deleting the old table.

You should back-up your database before doing this and ensure that your database is up-to-date first.`,
	Action: runRecreateTable,
}

func runRecreateTable(ctx *cli.Context) error {
	stdCtx, cancel := installSignals()
	defer cancel()

	// Redirect the default golog to here
	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.LoggerToWriter(log.GetLogger(log.DEFAULT).Info))

	debug := ctx.Bool("debug")
	setting.MustInstalled()
	setting.LoadDBSetting()

	if debug {
		setting.InitSQLLoggersForCli(log.DEBUG)
	} else {
		setting.InitSQLLoggersForCli(log.INFO)
	}

	setting.Database.LogSQL = debug
	if err := db.InitEngine(stdCtx); err != nil {
		fmt.Println(err)
		fmt.Println("Check if you are using the right config file. You can use a --config directive to specify one.")
		return nil
	}

	args := ctx.Args()
	names := make([]string, 0, ctx.NArg())
	for i := 0; i < ctx.NArg(); i++ {
		names = append(names, args.Get(i))
	}

	beans, err := db.NamesToBean(names...)
	if err != nil {
		return err
	}
	recreateTables := migrate_base.RecreateTables(beans...)

	return db.InitEngineWithMigration(stdCtx, func(x *xorm.Engine) error {
		if err := migrations.EnsureUpToDate(x); err != nil {
			return err
		}
		return recreateTables(x)
	})
}

func setupDoctorDefaultLogger(ctx *cli.Context, colorize bool) {
	// Silence the default loggers
	setupConsoleLogger(log.FATAL, log.CanColorStderr, os.Stderr)

	logFile := ctx.String("log-file")
	if logFile == "" {
		return // if no doctor log-file is set, do not show any log from default logger
	} else if logFile == "-" {
		setupConsoleLogger(log.TRACE, colorize, os.Stdout)
	} else {
		logFile, _ = filepath.Abs(logFile)
		writeMode := log.WriterMode{Level: log.TRACE, WriterOption: log.WriterFileOption{FileName: logFile}}
		writer, err := log.NewEventWriter("console-to-file", "file", writeMode)
		if err != nil {
			log.FallbackErrorf("unable to create file log writer: %v", err)
			return
		}
		log.GetManager().GetLogger(log.DEFAULT).ReplaceAllWriters(writer)
	}
}

func runDoctorCheck(ctx *cli.Context) error {
	stdCtx, cancel := installSignals()
	defer cancel()

	colorize := log.CanColorStdout
	if ctx.IsSet("color") {
		colorize = ctx.Bool("color")
	}

	setupDoctorDefaultLogger(ctx, colorize)

	// Finally redirect the default golang's log to here
	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.LoggerToWriter(log.GetLogger(log.DEFAULT).Info))

	if ctx.IsSet("list") {
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
		_, _ = w.Write([]byte("Default\tName\tTitle\n"))
		for _, check := range doctor.Checks {
			if check.IsDefault {
				_, _ = w.Write([]byte{'*'})
			}
			_, _ = w.Write([]byte{'\t'})
			_, _ = w.Write([]byte(check.Name))
			_, _ = w.Write([]byte{'\t'})
			_, _ = w.Write([]byte(check.Title))
			_, _ = w.Write([]byte{'\n'})
		}
		return w.Flush()
	}

	var checks []*doctor.Check
	if ctx.Bool("all") {
		checks = doctor.Checks
	} else if ctx.IsSet("run") {
		addDefault := ctx.Bool("default")
		names := ctx.StringSlice("run")
		for i, name := range names {
			names[i] = strings.ToLower(strings.TrimSpace(name))
		}

		for _, check := range doctor.Checks {
			if addDefault && check.IsDefault {
				checks = append(checks, check)
				continue
			}
			for _, name := range names {
				if name == check.Name {
					checks = append(checks, check)
					break
				}
			}
		}
	} else {
		for _, check := range doctor.Checks {
			if check.IsDefault {
				checks = append(checks, check)
			}
		}
	}

	return doctor.RunChecks(stdCtx, colorize, ctx.Bool("fix"), checks)
}
