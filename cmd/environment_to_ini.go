// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/unknwon/com"
	"github.com/urfave/cli"
	ini "gopkg.in/ini.v1"
)

// EnvironmentPrefix environment variables prefixed with this represent ini values to write
const EnvironmentPrefix = "GITEA__"

// CmdEnvironmentToIni represents the command to use a provided environment to update the configuration ini
var CmdEnvironmentToIni = cli.Command{
	Name:   "environment-to-ini",
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
		sectionName, keyName := DecodeSectionKey(eKey)
		if len(keyName) == 0 {
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

const escapeRegexpString = "_0[xX](([0-9a-fA-F][0-9a-fA-F])+)_"

var escapeRegex = regexp.MustCompile(escapeRegexpString)

// DecodeSectionKey will decode a portable string encoded Section__Key pair
// Portable strings are considered to be of the form [A-Z0-9_]*
// We will encode a disallowed value as the UTF8 byte string preceded by _0X and
// followed by _. E.g. _0X2C_ for a '-' and _0X2E_ for '.'
// Section and Key are separated by a plain '__'.
// The entire section can be encoded as a UTF8 byte string
func DecodeSectionKey(encoded string) (string, string) {
	section := ""
	key := ""

	inKey := false
	last := 0
	escapeStringIndices := escapeRegex.FindAllStringIndex(encoded, -1)
	for _, unescapeIdx := range escapeStringIndices {
		preceding := encoded[last:unescapeIdx[0]]
		if !inKey {
			if splitter := strings.Index(preceding, "__"); splitter > -1 {
				section += preceding[:splitter]
				inKey = true
				key += preceding[splitter+2:]
			} else {
				section += preceding
			}
		} else {
			key += preceding
		}
		toDecode := encoded[unescapeIdx[0]+3 : unescapeIdx[1]-1]
		decodedBytes := make([]byte, len(toDecode)/2)
		for i := 0; i < len(toDecode)/2; i++ {
			// Can ignore error here as we know these should be hexadecimal from the regexp
			byteInt, _ := strconv.ParseInt(toDecode[2*i:2*i+2], 16, 0)
			decodedBytes[i] = byte(byteInt)
		}
		if inKey {
			key += string(decodedBytes)
		} else {
			section += string(decodedBytes)
		}
		last = unescapeIdx[1]
	}
	remaining := encoded[last:]
	if !inKey {
		if splitter := strings.Index(remaining, "__"); splitter > -1 {
			section += remaining[:splitter]
			inKey = true
			key += remaining[splitter+2:]
		} else {
			section += remaining
		}
	} else {
		key += remaining
	}
	return section, key
}
