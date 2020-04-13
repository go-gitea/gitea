// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package cmd provides subcommands to the gitea binary - such as "web" or
// "admin".
package cmd

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli"
)

// argsSet checks that all the required arguments are set. args is a list of
// arguments that must be set in the passed Context.
func argsSet(c *cli.Context, args ...string) error {
	for _, a := range args {
		if !c.IsSet(a) {
			return errors.New(a + " is not set")
		}

		if util.IsEmptyString(a) {
			return errors.New(a + " is required")
		}
	}
	return nil
}

func initDB() error {
	return initDBDisableConsole(false)
}

func initDBDisableConsole(disableConsole bool) error {
	setting.NewContext()
	setting.InitDBConfig()

	setting.NewXORMLogService(disableConsole)
	if err := models.SetEngine(); err != nil {
		return fmt.Errorf("models.SetEngine: %v", err)
	}
	return nil
}
