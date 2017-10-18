// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

var (
	// CmdAdmin represents the available admin sub-command.
	CmdAdmin = cli.Command{
		Name:  "admin",
		Usage: "Perform admin operations on command line",
		Description: `Allow using internal logic of Gitea without hacking into the source code
to make automatic initialization process more smoothly`,
		Subcommands: []cli.Command{
			subcmdCreateUser,
			subcmdChangePassword,
		},
	}

	subcmdCreateUser = cli.Command{
		Name:   "create-user",
		Usage:  "Create a new user in database",
		Action: runCreateUser,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name",
				Value: "",
				Usage: "Username",
			},
			cli.StringFlag{
				Name:  "password",
				Value: "",
				Usage: "User password",
			},
			cli.StringFlag{
				Name:  "email",
				Value: "",
				Usage: "User email address",
			},
			cli.BoolFlag{
				Name:  "admin",
				Usage: "User is an admin",
			},
			cli.StringFlag{
				Name:  "config, c",
				Value: "custom/conf/app.ini",
				Usage: "Custom configuration file path",
			},
		},
	}

	subcmdChangePassword = cli.Command{
		Name:   "change-password",
		Usage:  "Change a user's password",
		Action: runChangePassword,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "username,u",
				Value: "",
				Usage: "The user to change password for",
			},
			cli.StringFlag{
				Name:  "password,p",
				Value: "",
				Usage: "New password to set for user",
			},
		},
	}
)

func runChangePassword(c *cli.Context) error {
	if !c.IsSet("password") {
		return fmt.Errorf("Password is not specified")
	} else if !c.IsSet("username") {
		return fmt.Errorf("Username is not specified")
	}

	setting.NewContext()
	models.LoadConfigs()

	setting.NewXORMLogService(false)
	if err := models.SetEngine(); err != nil {
		return fmt.Errorf("models.SetEngine: %v", err)
	}

	uname := c.String("username")
	user, err := models.GetUserByName(uname)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	user.Passwd = c.String("password")
	if user.Salt, err = models.GetUserSalt(); err != nil {
		return fmt.Errorf("%v", err)
	}
	user.EncodePasswd()
	if err := models.UpdateUserCols(user, "passwd", "salt"); err != nil {
		return fmt.Errorf("%v", err)
	}

	fmt.Printf("User '%s' password has been successfully updated!\n", uname)
	return nil
}

func runCreateUser(c *cli.Context) error {
	if !c.IsSet("name") {
		return fmt.Errorf("Username is not specified")
	} else if !c.IsSet("password") {
		return fmt.Errorf("Password is not specified")
	} else if !c.IsSet("email") {
		return fmt.Errorf("Email is not specified")
	}

	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	setting.NewContext()
	models.LoadConfigs()

	setting.NewXORMLogService(false)
	if err := models.SetEngine(); err != nil {
		return fmt.Errorf("models.SetEngine: %v", err)
	}

	if err := models.CreateUser(&models.User{
		Name:     c.String("name"),
		Email:    c.String("email"),
		Passwd:   c.String("password"),
		IsActive: true,
		IsAdmin:  c.Bool("admin"),
	}); err != nil {
		return fmt.Errorf("CreateUser: %v", err)
	}

	fmt.Printf("New user '%s' has been successfully created!\n", c.String("name"))
	return nil
}
