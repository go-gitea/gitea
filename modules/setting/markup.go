// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// ExternalMarkupParsers represents the external markup parsers
var (
	ExternalMarkupParsers []MarkupParser
)

// MarkupParser defines the external parser configured in ini
type MarkupParser struct {
	Enabled        bool
	MarkupName     string
	Command        string
	FileExtensions []string
	IsInputFile    bool
}

func newMarkup() {
	extensionReg := regexp.MustCompile(`\.\w`)
	for _, sec := range Cfg.Section("markup").ChildSections() {
		name := strings.TrimPrefix(sec.Name(), "markup.")
		if name == "" {
			log.Warn("name is empty, markup " + sec.Name() + "ignored")
			continue
		}

		extensions := sec.Key("FILE_EXTENSIONS").Strings(",")
		var exts = make([]string, 0, len(extensions))
		for _, extension := range extensions {
			if !extensionReg.MatchString(extension) {
				log.Warn(sec.Name() + " file extension " + extension + " is invalid. Extension ignored")
			} else {
				exts = append(exts, extension)
			}
		}

		if len(exts) == 0 {
			log.Warn(sec.Name() + " file extension is empty, markup " + name + " ignored")
			continue
		}

		command := sec.Key("RENDER_COMMAND").MustString("")
		if command == "" {
			log.Warn(" RENDER_COMMAND is empty, markup " + name + " ignored")
			continue
		}

		ExternalMarkupParsers = append(ExternalMarkupParsers, MarkupParser{
			Enabled:        sec.Key("ENABLED").MustBool(false),
			MarkupName:     name,
			FileExtensions: exts,
			Command:        command,
			IsInputFile:    sec.Key("IS_INPUT_FILE").MustBool(false),
		})
	}
}
