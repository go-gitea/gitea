// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v2"
)

// CmdBackup backup all data from database to fixtures files on dirPath
var CmdBackup = &cli.Command{
	Name:        "backup",
	Usage:       "Backup the Gitea database",
	Description: "A command to backup all data from database to fixtures files on dirPath",
	Action:      runBackup,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "dir-path",
			Value: "",
			Usage: "Directory path to save fixtures files",
		},
	},
}

func runBackup(ctx *cli.Context) error {
	stdCtx, cancel := installSignals()
	defer cancel()

	if err := initDB(stdCtx); err != nil {
		return err
	}

	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.Log.RootPath)
	log.Info("Configuration file: %s", setting.CustomConf)

	return db.BackupDatabaseAsFixtures(ctx.String("dir-path"))
}

var CmdRestore = &cli.Command{
	Name:        "restore",
	Usage:       "Restore the Gitea database",
	Description: "A command to restore all data from fixtures files on dirPath to database",
	Action:      runRestore,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "dir-path",
			Value: "",
			Usage: "Directory path to load fixtures files",
		},
	},
}

func runRestore(ctx *cli.Context) error {
	stdCtx, cancel := installSignals()
	defer cancel()

	if err := initDB(stdCtx); err != nil {
		return err
	}

	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.Log.RootPath)
	log.Info("Configuration file: %s", setting.CustomConf)

	return db.RestoreDatabase(ctx.String("dir-path"))
}
