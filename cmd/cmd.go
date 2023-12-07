// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package cmd provides subcommands to the gitea binary - such as "web" or
// "admin".
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli/v2"
)

// argsSet checks that all the required arguments are set. args is a list of
// arguments that must be set in the passed Context.
func argsSet(c *cli.Context, args ...string) error {
	for _, a := range args {
		if !c.IsSet(a) {
			return errors.New(a + " is not set")
		}

		if util.IsEmptyString(c.String(a)) {
			return errors.New(a + " is required")
		}
	}
	return nil
}

// confirm waits for user input which confirms an action
func confirm() (bool, error) {
	var response string

	_, err := fmt.Scanln(&response)
	if err != nil {
		return false, err
	}

	switch strings.ToLower(response) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, errors.New(response + " isn't a correct confirmation string")
	}
}

func initDB(ctx context.Context) error {
	setting.MustInstalled()
	setting.LoadDBSetting()
	setting.InitSQLLoggersForCli(log.INFO)

	if setting.Database.Type == "" {
		log.Fatal(`Database settings are missing from the configuration file: %q.
Ensure you are running in the correct environment or set the correct configuration file with -c.
If this is the intended configuration file complete the [database] section.`, setting.CustomConf)
	}
	if err := db.InitEngine(ctx); err != nil {
		return fmt.Errorf("unable to initialize the database using the configuration in %q. Error: %w", setting.CustomConf, err)
	}
	return nil
}

func installSignals() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// install notify
		signalChannel := make(chan os.Signal, 1)

		signal.Notify(
			signalChannel,
			syscall.SIGINT,
			syscall.SIGTERM,
		)
		select {
		case <-signalChannel:
		case <-ctx.Done():
		}
		cancel()
		signal.Reset()
	}()

	return ctx, cancel
}

func setupConsoleLogger(level log.Level, colorize bool, out io.Writer) {
	if out != os.Stdout && out != os.Stderr {
		panic("setupConsoleLogger can only be used with os.Stdout or os.Stderr")
	}

	writeMode := log.WriterMode{
		Level:        level,
		Colorize:     colorize,
		WriterOption: log.WriterConsoleOption{Stderr: out == os.Stderr},
	}
	writer := log.NewEventWriterConsole("console-default", writeMode)
	log.GetManager().GetLogger(log.DEFAULT).ReplaceAllWriters(writer)
}

func globalBool(c *cli.Context, name string) bool {
	for _, ctx := range c.Lineage() {
		if ctx.Bool(name) {
			return true
		}
	}
	return false
}

// PrepareConsoleLoggerLevel by default, use INFO level for console logger, but some sub-commands (for git/ssh protocol) shouldn't output any log to stdout.
// Any log appears in git stdout pipe will break the git protocol, eg: client can't push and hangs forever.
func PrepareConsoleLoggerLevel(defaultLevel log.Level) func(*cli.Context) error {
	return func(c *cli.Context) error {
		level := defaultLevel
		if globalBool(c, "quiet") {
			level = log.FATAL
		}
		if globalBool(c, "debug") || globalBool(c, "verbose") {
			level = log.TRACE
		}
		log.SetConsoleLogger(log.DEFAULT, "console-default", level)
		return nil
	}
}
