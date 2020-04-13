// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"gopkg.in/ini.v1"
)

// ExternalMarkupParsers represents the external markup parsers
var (
	ExternalMarkupParsers  []MarkupParser
	ExternalSanitizerRules []MarkupSanitizerRule
)

// MarkupParser defines the external parser configured in ini
type MarkupParser struct {
	Enabled        bool
	MarkupName     string
	Command        string
	FileExtensions []string
	IsInputFile    bool
}

// MarkupSanitizerRule defines the policy for whitelisting attributes on
// certain elements.
type MarkupSanitizerRule struct {
	Element   string
	AllowAttr string
	Regexp    *regexp.Regexp
}

func newMarkup() {
	for _, sec := range Cfg.Section("markup").ChildSections() {
		name := strings.TrimPrefix(sec.Name(), "markup.")
		if name == "" {
			log.Warn("name is empty, markup " + sec.Name() + "ignored")
			continue
		}

		if name == "sanitizer" {
			newMarkupSanitizer(name, sec)
		} else {
			newMarkupRenderer(name, sec)
		}
	}
}

func newMarkupSanitizer(name string, sec *ini.Section) {
	haveElement := sec.HasKey("ELEMENT")
	haveAttr := sec.HasKey("ALLOW_ATTR")
	haveRegexp := sec.HasKey("REGEXP")

	if !haveElement && !haveAttr && !haveRegexp {
		log.Warn("Skipping empty section: markup.%s.", name)
		return
	}

	if !haveElement || !haveAttr || !haveRegexp {
		log.Error("Missing required keys from markup.%s. Must have all three of ELEMENT, ALLOW_ATTR, and REGEXP defined!", name)
		return
	}

	elements := sec.Key("ELEMENT").ValueWithShadows()
	allowAttrs := sec.Key("ALLOW_ATTR").ValueWithShadows()
	regexps := sec.Key("REGEXP").ValueWithShadows()

	if len(elements) != len(allowAttrs) ||
		len(elements) != len(regexps) {
		log.Error("All three keys in markup.%s (ELEMENT, ALLOW_ATTR, REGEXP) must be defined the same number of times! Got %d, %d, and %d respectively.", name, len(elements), len(allowAttrs), len(regexps))
		return
	}

	ExternalSanitizerRules = make([]MarkupSanitizerRule, 0, len(elements))

	for index, pattern := range regexps {
		if pattern == "" {
			rule := MarkupSanitizerRule{
				Element:   elements[index],
				AllowAttr: allowAttrs[index],
				Regexp:    nil,
			}
			ExternalSanitizerRules = append(ExternalSanitizerRules, rule)
			continue
		}

		// Validate when parsing the config that this is a valid regular
		// expression. Then we can use regexp.MustCompile(...) later.
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			log.Error("In module.%s: REGEXP at definition %d failed to compile: %v", name, index+1, err)
			continue
		}

		rule := MarkupSanitizerRule{
			Element:   elements[index],
			AllowAttr: allowAttrs[index],
			Regexp:    compiled,
		}
		ExternalSanitizerRules = append(ExternalSanitizerRules, rule)
	}
}

func newMarkupRenderer(name string, sec *ini.Section) {
	extensionReg := regexp.MustCompile(`\.\w`)

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
		return
	}

	command := sec.Key("RENDER_COMMAND").MustString("")
	if command == "" {
		log.Warn(" RENDER_COMMAND is empty, markup " + name + " ignored")
		return
	}

	ExternalMarkupParsers = append(ExternalMarkupParsers, MarkupParser{
		Enabled:        sec.Key("ENABLED").MustBool(false),
		MarkupName:     name,
		FileExtensions: exts,
		Command:        command,
		IsInputFile:    sec.Key("IS_INPUT_FILE").MustBool(false),
	})
}
