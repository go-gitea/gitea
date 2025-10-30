// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v3"
)

func cmdConfig() *cli.Command {
	subcmdConfigEditIni := &cli.Command{
		Name:  "edit-ini",
		Usage: "Load an existing INI file, apply environment variables, keep specified keys, and output to a new INI file.",
		Description: `
Help users to edit the Gitea configuration INI file.

# Keep Specified Keys

If you need to re-create the configuration file with only a subset of keys,
you can provide an INI template file for the kept keys and use the "--config-keep-keys" flag.
For example, if a helm chart needs to reset the settings and only keep SECRET_KEY,
it can use a template file (only keys take effect, values are ignored):

  [security]
  SECRET_KEY=

$ ./gitea config edit-ini --config app-old.ini --config-keep-keys app-keys.ini --out app-new.ini

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

$ ./gitea config edit-ini --config app.ini --config-keep-keys app-keys.ini --apply-env {--in-place|--out app-new.ini}
`,
		Flags: []cli.Flag{
			// "--config" flag is provided by global flags, and this flag is also used by "environment-to-ini" script wrapper
			// "--in-place" is also used by "environment-to-ini" script wrapper for its old behavior: always overwrite the existing config file
			&cli.BoolFlag{
				Name:  "in-place",
				Usage: "Output to the same config file as input. This flag will be ignored if --out is set.",
			},
			&cli.StringFlag{
				Name:  "config-keep-keys",
				Usage: "An INI template file containing keys for keeping. Only the keys defined in the INI template will be kept from old config. If not set, all keys will be kept.",
			},
			&cli.BoolFlag{
				Name:  "apply-env",
				Usage: "Apply all GITEA__* variables from the environment to the config.",
			},
			&cli.StringFlag{
				Name:  "out",
				Usage: "Destination config file to write to.",
			},
		},
		Action: runConfigEditIni,
	}

	return &cli.Command{
		Name:  "config",
		Usage: "Manage Gitea configuration",
		Commands: []*cli.Command{
			subcmdConfigEditIni,
		},
	}
}

func runConfigEditIni(_ context.Context, c *cli.Command) error {
	// the config system may change the environment variables, so get a copy first, to be used later
	env := append([]string{}, os.Environ()...)

	// don't use the guessed setting.CustomConf, instead, require the user to provide --config explicitly
	if !c.IsSet("config") {
		return errors.New("flag is required but not set: --config")
	}
	configFileIn := c.String("config")

	cfgIn, err := setting.NewConfigProviderFromFile(configFileIn)
	if err != nil {
		return fmt.Errorf("failed to load config file %q: %v", configFileIn, err)
	}

	// determine output config file: use "--out" flag or use "--in-place" flag to overwrite input file
	inPlace := c.Bool("in-place")
	configFileOut := c.String("out")
	if configFileOut == "" {
		if !inPlace {
			return errors.New("either --in-place or --out must be specified")
		}
		configFileOut = configFileIn // in-place edit
	}

	needWriteOut := configFileOut != configFileIn

	cfgOut := cfgIn
	configKeepKeys := c.String("config-keep-keys")
	if configKeepKeys != "" {
		needWriteOut = true
		cfgOut, err = setting.NewConfigProviderFromFile(configKeepKeys)
		if err != nil {
			return fmt.Errorf("failed to load config-keep-keys template file %q: %v", configKeepKeys, err)
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
		err = cfgOut.SaveTo(configFileOut)
		if err != nil {
			return err
		}
	}
	return nil
}
