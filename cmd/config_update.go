// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli/v3"
)

func cmdConfig() *cli.Command {
	subcmdConfigUpdateIni := &cli.Command{
		Name:  "update-ini",
		Usage: "Load an existing INI file, apply environment variables, keep specified keys, and output to a new INI file.",
		Description: `
Help users to update the gitea configuration INI file:
* Keep specified keys to for the new INI file.
* Map environment variables to values in the INI.

# Keep Specified Keys

If you need to re-create the configuration file with only a subset of keys,
you can provide an INI template file and use the "--config-key-template" flag.
For example, if a helm chart needs to reset the settings and only keep SECRET_KEY,
it can use a template file like:

  [security]
  SECRET_KEY=

$ ./gitea config update-ini --config app-old.ini --config-key-template app-template.ini --out app-new.ini

# Map Environment Variables to INI Configuration

Environment variables of the form "GITEA__section_name__KEY_NAME"
will be mapped to the ini section "[section_name]" and the key
"KEY_NAME" with the value as provided.

Environment variables of the form "GITEA__section_name__KEY_NAME__FILE"
will be mapped to the ini section "[section_name]" and the key
"KEY_NAME" with the value loaded from the specified file.

Environment variable keys can only contain characters "0-9A-Z_",
if a section or key name contains dot ".", it needs to be escaped as _0x2E_.
For example, to apply this config:

	[git.config]
	foo.bar=val

$ export GITEA__git_0x2E_config__foo_0x2E_bar=val

# Put All Together

$ ./gitea config update-ini --config app.ini --config-key-template app-template.ini --apply-env
`,
		Flags: []cli.Flag{
			// "--config" flag is provided by global flags, and this flag is also used by "environment-to-ini" script wrapper
			&cli.StringFlag{
				Name:  "config-key-template",
				Usage: "An INI template file containing keys for keeping. Only the keys defined in the INI template will be kept from old config. If not set, all keys will be kept.",
			},
			&cli.BoolFlag{
				Name:  "apply-env",
				Usage: "Apply all GITEA__* variables from the environment to the config.",
			},
			&cli.StringFlag{
				Name:  "out",
				Usage: "Destination config file to write to. If not set, will overwrite the source config file.",
			},
		},
		Action: runConfigUpdateIni,
	}

	return &cli.Command{
		Name:  "config",
		Usage: "Manage Gitea configuration",
		Commands: []*cli.Command{
			subcmdConfigUpdateIni,
		},
	}
}

func runConfigUpdateIni(_ context.Context, c *cli.Command) error {
	// the config system may change the environment variables, so get a copy first, to be used later
	env := append([]string{}, os.Environ()...)
	if !c.IsSet("config") {
		return errors.New("flag is required but not set: --config")
	}

	configFileIn := c.String("config")
	cfgIn, err := setting.NewConfigProviderFromFile(configFileIn)
	if err != nil {
		return fmt.Errorf("failed to load config file %q: %v", configFileIn, err)
	}

	configFileOut := c.String("out")
	configFileOut = util.IfZero(configFileOut, configFileIn)
	needWriteOut := configFileOut != configFileIn

	cfgOut := cfgIn
	if c.IsSet("config-key-template") {
		needWriteOut = true
		configKeepTemplate := c.String("config-key-template")
		cfgOut, err = setting.NewConfigProviderFromFile(configKeepTemplate)
		if err != nil {
			return fmt.Errorf("failed to load config keep template file %q: %v", configKeepTemplate, err)
		}

		for _, secOut := range cfgOut.Sections() {
			for _, keyOut := range secOut.Keys() {
				secIn := cfgIn.Section(secOut.Name())
				keyIn := setting.ConfigSectionKey(secIn, keyOut.Name())
				if keyIn != nil {
					keyOut.SetValue(keyIn.String())
				} else {
					secOut.DeleteKey(keyOut.Name())
				}
			}
			if len(secOut.Keys()) == 0 {
				cfgOut.DeleteSection(secOut.Name())
			}
		}
	}

	if c.Bool("apply-env") {
		if setting.EnvironmentToConfig(cfgOut, env) {
			needWriteOut = true
		}
	}

	if needWriteOut {
		_, _ = fmt.Fprintf(c.Writer, "Saving config to: %q\n", configFileOut)
		err = cfgOut.SaveTo(configFileOut)
		if err != nil {
			return err
		}
	}
	return nil
}
