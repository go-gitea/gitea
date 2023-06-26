// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

// EnvironmentPrefix environment variables prefixed with this represent ini values to write
const EnvironmentPrefix = "GITEA"

func main() {
	app := cli.NewApp()
	app.Name = "environment-to-ini"
	app.Usage = "Use provided environment to update configuration ini"
	app.Description = `As a helper to allow docker users to update the gitea configuration
	through the environment, this command allows environment variables to
	be mapped to values in the ini.

	Environment variables of the form "GITEA__SECTION_NAME__KEY_NAME"
	will be mapped to the ini section "[section_name]" and the key
	"KEY_NAME" with the value as provided.

	Environment variables of the form "GITEA__SECTION_NAME__KEY_NAME__FILE"
	will be mapped to the ini section "[section_name]" and the key
	"KEY_NAME" with the value loaded from the specified file.

	Environment variables are usually restricted to a reduced character
	set "0-9A-Z_" - in order to allow the setting of sections with
	characters outside of that set, they should be escaped as following:
	"_0X2E_" for ".". The entire section and key names can be escaped as
	a UTF8 byte string if necessary. E.g. to configure:

		"""
		...
		[log.console]
		COLORIZE=false
		STDERR=true
		...
		"""

	You would set the environment variables: "GITEA__LOG_0x2E_CONSOLE__COLORIZE=false"
	and "GITEA__LOG_0x2E_CONSOLE__STDERR=false". Other examples can be found
	on the configuration cheat sheet.`
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "custom-path, C",
			Value: setting.CustomPath,
			Usage: "Custom path file path",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: setting.CustomConf,
			Usage: "Custom configuration file path",
		},
		cli.StringFlag{
			Name:  "work-path, w",
			Value: setting.AppWorkPath,
			Usage: "Set the gitea working path",
		},
		cli.StringFlag{
			Name:  "out, o",
			Value: "",
			Usage: "Destination file to write to",
		},
		cli.BoolFlag{
			Name:  "clear",
			Usage: "Clears the matched variables from the environment",
		},
		cli.StringFlag{
			Name:  "prefix, p",
			Value: EnvironmentPrefix,
			Usage: "Environment prefix to look for - will be suffixed by __ (2 underscores)",
		},
	}
	app.Action = runEnvironmentToIni
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal("Failed to run app with %s: %v", os.Args, err)
	}
}

func runEnvironmentToIni(c *cli.Context) error {
	setting.InitWorkPathAndCfgProvider(os.Getenv, setting.ArgWorkPathAndCustomConf{
		WorkPath:   c.String("work-path"),
		CustomPath: c.String("custom-path"),
		CustomConf: c.String("config"),
	})

	cfg, err := setting.NewConfigProviderFromFile(setting.CustomConf)
	if err != nil {
		log.Fatal("Failed to load custom conf '%s': %v", setting.CustomConf, err)
	}

	prefixGitea := c.String("prefix") + "__"
	suffixFile := "__FILE"
	changed := setting.EnvironmentToConfig(cfg, prefixGitea, suffixFile, os.Environ())

	// try to save the config file
	destination := c.String("out")
	if len(destination) == 0 {
		destination = setting.CustomConf
	}
	if destination != setting.CustomConf || changed {
		log.Info("Settings saved to: %q", destination)
		err = cfg.SaveTo(destination)
		if err != nil {
			return err
		}
	}

	// clear Gitea's specific environment variables if requested
	if c.Bool("clear") {
		for _, kv := range os.Environ() {
			idx := strings.IndexByte(kv, '=')
			if idx < 0 {
				continue
			}
			eKey := kv[:idx]
			if strings.HasPrefix(eKey, prefixGitea) {
				_ = os.Unsetenv(eKey)
			}
		}
	}

	return nil
}
