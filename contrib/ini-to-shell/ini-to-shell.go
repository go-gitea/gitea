// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	golog "log"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "ini-to-shell"
	app.Usage = "Extract settings from an existing configuration ini"
	app.Description = `This is the counterpart to environment-to-ini.
	It allows extracting settings from an existing ini file for further
	processing in e.g. a shell.

	Since it is not possible to define an environment variable for the
	parent process, this script simply echoes the value.

		"""
		./ini-to-shell -c /path/to/app.ini -s '<section name>' -k '<key name>'
		"""

	Section and key name are case sensitive and MUST match with the ini content.`
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "custom-path",
			Aliases: []string{"C"},
			Value:   setting.CustomPath,
			Usage:   "Custom path file path",
		},
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   setting.CustomConf,
			Usage:   "Custom configuration file path",
		},
		&cli.StringFlag{
			Name:    "work-path",
			Aliases: []string{"w"},
			Value:   setting.AppWorkPath,
			Usage:   "Set the gitea working path",
		},
		&cli.StringFlag{
			Name:    "section",
			Aliases: []string{"s"},
			Value:   "",
			Usage:   "Section name to search the given key (leave empty for default/root section)",
		},
		&cli.StringFlag{
			Name:     "key",
			Aliases:  []string{"k"},
			Required: true,
			Value:    "",
			Usage:    "Key name to extract the value from",
		},
	}
	app.Action = runIniToShell
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal("Failed to run app with %s: %v", os.Args, err)
	}
}

func runIniToShell(c *cli.Context) error {
	setting.InitWorkPathAndCfgProvider(os.Getenv, setting.ArgWorkPathAndCustomConf{
		WorkPath:   c.String("work-path"),
		CustomPath: c.String("custom-path"),
		CustomConf: c.String("config"),
	})

	cfg, err := setting.NewConfigProviderFromFile(setting.CustomConf)
	if err != nil {
		log.Fatal("Failed to load custom conf '%s': %v", setting.CustomConf, err)
	}

	sName := c.String("section")
	kName := c.String("key")

	section, err := cfg.GetSection(sName)
	if err != nil {
		log.Fatal("Failed to load section '%s': %v", sName, err)
	}

	if !section.HasKey(kName) {
		log.Fatal("Section '%s' does not have key '%s'", sName, kName)
	}

	golog.SetOutput(os.Stdout)
	golog.SetFlags(golog.Flags() &^ (golog.Ldate | golog.Ltime))
	golog.Println(section.Key(kName).Value())

	return nil
}
