// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/Unknwon/com"
	"github.com/urfave/cli"
	ini "gopkg.in/ini.v1"
)

// EnvironmentPrefix environment variables prefixed with this represent ini values to write
const EnvironmentPrefix = "GITEA:"

// Separator is the character that will separate section from key name
const Separator = ":"

// CmdEnvironmentToIni represents the command to use a provided environment to update the configuration ini
var CmdEnvironmentToIni = cli.Command{
	Name:   "enviroment-to-ini",
	Usage:  "Use provided environment to update configuration ini",
	Action: runEnvironmentToIni,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "out, o",
			Value: "",
			Usage: "Destination file to write to",
		},
	},
}

func runEnvironmentToIni(c *cli.Context) error {
	cfg := ini.Empty()
	if com.IsFile(setting.CustomConf) {
		if err := cfg.Append(setting.CustomConf); err != nil {
			log.Fatal("Failed to load custom conf '%s': %v", setting.CustomConf, err)
		}
	} else {
		log.Warn("Custom config '%s' not found, ignore this if you're running first time", setting.CustomConf)
	}
	cfg.NameMapper = ini.AllCapsUnderscore

	for _, kv := range os.Environ() {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		eKey := kv[:idx]
		value := kv[idx+1:]
		if !strings.HasPrefix(eKey, EnvironmentPrefix) {
			continue
		}
		parts := strings.Split(eKey, Separator)
		if len(parts) != 3 {
			continue
		}
		sectionName := parts[1]
		keyName := parts[2]
		if len(sectionName) == 0 || len(keyName) == 0 {
			continue
		}
		section, err := cfg.GetSection(sectionName)
		if err != nil {
			section, err = cfg.NewSection(sectionName)
			if err != nil {
				log.Error("Error creating section: %s : %v", sectionName, err)
				continue
			}
		}
		key := section.Key(keyName)
		if key == nil {
			key, err = section.NewKey(keyName, value)
			if err != nil {
				log.Error("Error creating key: %s in section: %s with value: %s : %v", keyName, sectionName, value, err)
				continue
			}
		}
		key.SetValue(value)
	}
	destination := c.String("out")
	if len(destination) == 0 {
		destination = setting.CustomConf
	}
	err := cfg.SaveTo(destination)
	return err
}
