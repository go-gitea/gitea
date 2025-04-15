// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

// Check represents a Doctor check
type Check struct {
	Title                      string
	Name                       string
	IsDefault                  bool
	Run                        func(ctx context.Context, logger log.Logger, autofix bool) error
	AbortIfFailed              bool
	SkipDatabaseInitialization bool
	Priority                   int
	InitStorage                bool
}

func initDBSkipLogger(ctx context.Context) error {
	setting.MustInstalled()
	setting.LoadDBSetting()
	if err := db.InitEngine(ctx); err != nil {
		return fmt.Errorf("db.InitEngine: %w", err)
	}
	// some doctor sub-commands need to use git command
	if err := git.InitFull(ctx); err != nil {
		return fmt.Errorf("git.InitFull: %w", err)
	}
	return nil
}

type doctorCheckLogger struct {
	colorize bool
}

var _ log.BaseLogger = (*doctorCheckLogger)(nil)

func (d *doctorCheckLogger) Log(skip int, event *log.Event, format string, v ...any) {
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", v...)
}

func (d *doctorCheckLogger) GetLevel() log.Level {
	return log.TRACE
}

type doctorCheckStepLogger struct {
	colorize bool
}

var _ log.BaseLogger = (*doctorCheckStepLogger)(nil)

func (d *doctorCheckStepLogger) Log(skip int, event *log.Event, format string, v ...any) {
	levelChar := fmt.Sprintf("[%s]", strings.ToUpper(event.Level.String()[0:1]))
	var levelArg any = levelChar
	if d.colorize {
		levelArg = log.NewColoredValue(levelChar, event.Level.ColorAttributes()...)
	}
	args := append([]any{levelArg}, v...)
	_, _ = fmt.Fprintf(os.Stdout, " - %s "+format+"\n", args...)
}

func (d *doctorCheckStepLogger) GetLevel() log.Level {
	return log.TRACE
}

// Checks is the list of available commands
var Checks []*Check

// RunChecks runs the doctor checks for the provided list
func RunChecks(ctx context.Context, colorize, autofix bool, checks []*Check) error {
	SortChecks(checks)
	// the checks output logs by a special logger, they do not use the default logger
	logger := log.BaseLoggerToGeneralLogger(&doctorCheckLogger{colorize: colorize})
	loggerStep := log.BaseLoggerToGeneralLogger(&doctorCheckStepLogger{colorize: colorize})
	dbIsInit := false
	storageIsInit := false
	for i, check := range checks {
		if !dbIsInit && !check.SkipDatabaseInitialization {
			// Only open database after the most basic configuration check
			if err := initDBSkipLogger(ctx); err != nil {
				logger.Error("Error whilst initializing the database: %v", err)
				logger.Error("Check if you are using the right config file. You can use a --config directive to specify one.")
				return nil
			}
			dbIsInit = true
		}
		if !storageIsInit && check.InitStorage {
			if err := storage.Init(); err != nil {
				logger.Error("Error whilst initializing the storage: %v", err)
				logger.Error("Check if you are using the right config file. You can use a --config directive to specify one.")
				return nil
			}
			storageIsInit = true
		}
		logger.Info("\n[%d] %s", i+1, check.Title)
		if err := check.Run(ctx, loggerStep, autofix); err != nil {
			if check.AbortIfFailed {
				logger.Critical("FAIL")
				return err
			}
			logger.Error("ERROR")
		} else {
			logger.Info("OK")
		}
	}
	logger.Info("\nAll done (checks: %d).", len(checks))
	return nil
}

// Register registers a command with the list
func Register(command *Check) {
	Checks = append(Checks, command)
}

func SortChecks(checks []*Check) {
	sort.SliceStable(checks, func(i, j int) bool {
		if checks[i].Priority == checks[j].Priority {
			return checks[i].Name < checks[j].Name
		}
		if checks[i].Priority == 0 {
			return false
		}
		return checks[i].Priority < checks[j].Priority
	})
}
