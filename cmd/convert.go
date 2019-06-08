// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

// CmdConvert represents the available convert sub-command.
var CmdConvert = cli.Command{
	Name:        "convert",
	Usage:       "Convert the database",
	Description: "This is a command for converting the database.",
	Action:      runConvert,
}

func runConvert(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	models.LoadConfigs()

	if models.DbCfg.Type != "mysql" {
		fmt.Println("This command only be used with mysql database")
		return nil
	}

	if err := models.ConvertUtf8ToUtf8mb4(); err != nil {
		log.Fatal("Failed to comvert database from utf8 to utf8mb4: %v", err)
		return err
	}

	fmt.Println("Convert successfully, please confirm your database connstr with utf8mb4")

	return nil
}
