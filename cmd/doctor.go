// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"io"
	golog "log"
	"os"
	"strings"
	"text/tabwriter"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/migrations"
	migrate_base "code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/doctor"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
	"xorm.io/xorm"
)

// CmdDoctor represents the available doctor sub-command.
var CmdDoctor = cli.Command{
	Name:        "doctor",
	Usage:       "Diagnose and optionally fix problems",
	Description: "Helper commands to diagnose problems with the current Gitea instance according to the given configuration. Some problems can optionally be fixed by modifying the database or data storage.",
	Subcommands: []cli.Command{
		cmdDoctorCheck,
		cmdRecreateTable,
		CmdConvert,
	},
}

var cmdRecreateTable = cli.Command{
	Name:      "recreate-table",
	Usage:     "Recreate tables from XORM definitions and copy the data.",
	ArgsUsage: "[TABLE]... : (TABLEs to recreate - leave blank for all)",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Print SQL commands sent",
		},
	},
	Description: `The database definitions Gitea uses change across versions, sometimes changing default values and leaving old unused columns.

This command will cause Xorm to recreate tables, copying over the data and deleting the old table.

You should back-up your database before doing this and ensure that your database is up-to-date first.`,
	Action: runRecreateTable,
}

var cmdDoctorCheck = cli.Command{
	Name:      "check",
	Usage:     "Runs doctor check(s)",
	ArgsUsage: "[check-name]... : check(s) to run - leave blank to just run the default checks.\n\n",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "fix",
			Usage: "Automatically fix what we can",
		},
		cli.StringFlag{
			Name:  "log-file",
			Usage: `Name of the log file (if empty defaults to: "doctor.log"). Set to "-" to output to stdout, leave unset to not run logs`,
		},
		cli.BoolFlag{
			Name:  "color, H",
			Usage: "Use color for outputted information",
		},
		cli.BoolFlag{
			Name:  "verbose, V",
			Usage: "log to stdout, (shorthand for --log-file=-",
		},
	},
	Action: runDoctorCheck,
}

func init() {
	sb := new(strings.Builder)
	writeChecks(sb)

	CmdDoctor.Subcommands[0].ArgsUsage += sb.String()
}

func writeChecks(sb io.Writer) {
	_, _ = sb.Write([]byte("CHECKS:\n"))
	w := tabwriter.NewWriter(sb, 0, 8, 1, ' ', 0)
	_, _ = w.Write([]byte("  \tlist\tPrints list of available checks\n"))
	_, _ = w.Write([]byte("  \tall\tRuns all available checks\n"))
	_, _ = w.Write([]byte("  \tdefault\tRuns checks marked with (*) below\n"))
	for _, check := range doctor.Checks {
		_, _ = w.Write([]byte("  \t"))
		_, _ = w.Write([]byte(check.Name))
		_, _ = w.Write([]byte{'\t'})
		if check.IsDefault {
			_, _ = w.Write([]byte("(*) "))
		}
		_, _ = w.Write([]byte(check.Title))
		_, _ = w.Write([]byte{'\n'})
	}
	_ = w.Flush()
}

func runDoctorCheck(ctx *cli.Context) error {
	stdCtx, cancel := installSignals()
	defer cancel()

	// Silence the default loggers
	log.DelNamedLogger("console")
	log.DelNamedLogger(log.DEFAULT)

	// Now setup our logger
	setDoctorLogger(ctx)

	colorize := log.CanColorStdout
	if ctx.IsSet("color") {
		colorize = ctx.Bool("color")
	}

	// Finally redirect the default golog to here
	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.NewLoggerAsWriter("INFO", log.GetLogger(log.DEFAULT)))

	// Now we can set up our own logger to return information about what the doctor is doing
	if err := log.NewNamedLogger("doctorouter",
		0,
		"console",
		"console",
		fmt.Sprintf(`{"level":"INFO","stacktracelevel":"NONE","colorize":%t,"flags":-1}`, colorize)); err != nil {
		fmt.Println(err)
		return err
	}

	logger := log.GetLogger("doctorouter")
	defer logger.Close()

	var checks []*doctor.Check
	if ctx.NArg() > 0 {
		names := make([]string, 0, ctx.NArg())
		args := ctx.Args()
		for i := 0; i < ctx.NArg(); i++ {
			names = append(names, args.Get(i))
		}

		addDefault := false
		all := false
		for i, name := range names {
			names[i] = strings.ToLower(strings.TrimSpace(name))
			switch names[i] {
			case "default":
				addDefault = true
			case "list":
				sb := new(strings.Builder)
				writeChecks(sb)
				logger.Info("%s", log.NewColoredValue(sb.String(), log.Reset))
			case "all":
				all = true
			}
		}

		if all {
			checks = doctor.Checks
		} else {
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
		}
	} else {
		for _, check := range doctor.Checks {
			if check.IsDefault {
				checks = append(checks, check)
			}
		}
	}

	return doctor.RunChecks(stdCtx, logger, ctx.Bool("fix"), checks)
}

func runRecreateTable(ctx *cli.Context) error {
	// Redirect the default golog to here
	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.NewLoggerAsWriter("INFO", log.GetLogger(log.DEFAULT)))

	setting.LoadFromExisting()
	setting.InitDBConfig()

	setting.EnableXORMLog = ctx.Bool("debug")
	setting.Database.LogSQL = ctx.Bool("debug")
	setting.Cfg.Section("log").Key("XORM").SetValue(",")

	setting.NewXORMLogService(!ctx.Bool("debug"))
	stdCtx, cancel := installSignals()
	defer cancel()

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

func setDoctorLogger(ctx *cli.Context) {
	logFile := ctx.String("log-file")
	if ctx.IsSet("log-file") {
		if logFile == "" {
			// verbose is set and log-file="" then assume that we mean --log-file=-
			if ctx.Bool("verbose") {
				logFile = "-"
			} else {
				logFile = "doctor.log"
			}
		}
	} else if ctx.Bool("verbose") {
		logFile = "-"
	}
	colorize := log.CanColorStdout
	if ctx.IsSet("color") {
		colorize = ctx.Bool("color")
	}

	if len(logFile) == 0 {
		log.NewLogger(1000, "doctor", "console", fmt.Sprintf(`{"level":"NONE","stacktracelevel":"NONE","colorize":%t}`, colorize))
		return
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}

		err, ok := recovered.(error)
		if !ok {
			panic(recovered)
		}
		if errors.Is(err, os.ErrPermission) {
			fmt.Fprintf(os.Stderr, "ERROR: Unable to write logs to provided file due to permissions error: %s\n       %v\n", logFile, err)
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: Unable to write logs to provided file: %s\n       %v\n", logFile, err)
		}
		fmt.Fprintf(os.Stderr, "WARN: Logging will be disabled\n       Use `--log-file` to configure log file location\n")
		log.NewLogger(1000, "doctor", "console", fmt.Sprintf(`{"level":"NONE","stacktracelevel":"NONE","colorize":%t}`, colorize))
	}()

	if logFile == "-" {
		log.NewLogger(1000, "doctor", "console", fmt.Sprintf(`{"level":"trace","stacktracelevel":"NONE","colorize":%t}`, colorize))
	} else {
		log.NewLogger(1000, "doctor", "file", fmt.Sprintf(`{"filename":%q,"level":"trace","stacktracelevel":"NONE"}`, logFile))
	}
}
