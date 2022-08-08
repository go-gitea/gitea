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

// ExternalMarkupRenderers represents the external markup renderers
var (
	ExternalMarkupRenderers    []*MarkupRenderer
	ExternalSanitizerRules     []MarkupSanitizerRule
	MermaidMaxSourceCharacters int
)

const (
	RenderContentModeSanitized   = "sanitized"
	RenderContentModeNoSanitizer = "no-sanitizer"
	RenderContentModeIframe      = "iframe"
)

// MarkupRenderer defines the external parser configured in ini
type MarkupRenderer struct {
	Enabled                    bool
	MarkupName                 string
	Command                    string
	FileExtensions             []string
	IsInputFile                bool
	NeedPostProcess            bool
	MarkupSanitizerRules       []MarkupSanitizerRule
	RenderContentMode          string
	RenderContentIframeSandbox string
	RenderContentExternalCSP   string
}

// MarkupSanitizerRule defines the policy for whitelisting attributes on
// certain elements.
type MarkupSanitizerRule struct {
	Element            string
	AllowAttr          string
	Regexp             *regexp.Regexp
	AllowDataURIImages bool
}

func newMarkup() {
	MermaidMaxSourceCharacters = Cfg.Section("markup").Key("MERMAID_MAX_SOURCE_CHARACTERS").MustInt(5000)
	ExternalMarkupRenderers = make([]*MarkupRenderer, 0, 10)
	ExternalSanitizerRules = make([]MarkupSanitizerRule, 0, 10)

	for _, sec := range Cfg.Section("markup").ChildSections() {
		name := strings.TrimPrefix(sec.Name(), "markup.")
		if name == "" {
			log.Warn("name is empty, markup " + sec.Name() + "ignored")
			continue
		}

		if name == "sanitizer" || strings.HasPrefix(name, "sanitizer.") {
			newMarkupSanitizer(name, sec)
		} else {
			newMarkupRenderer(name, sec)
		}
	}
}

func newMarkupSanitizer(name string, sec *ini.Section) {
	rule, ok := createMarkupSanitizerRule(name, sec)
	if ok {
		if strings.HasPrefix(name, "sanitizer.") {
			names := strings.SplitN(strings.TrimPrefix(name, "sanitizer."), ".", 2)
			name = names[0]
		}
		for _, renderer := range ExternalMarkupRenderers {
			if name == renderer.MarkupName {
				renderer.MarkupSanitizerRules = append(renderer.MarkupSanitizerRules, rule)
				return
			}
		}
		ExternalSanitizerRules = append(ExternalSanitizerRules, rule)
	}
}

func createMarkupSanitizerRule(name string, sec *ini.Section) (MarkupSanitizerRule, bool) {
	var rule MarkupSanitizerRule

	ok := false
	if sec.HasKey("ALLOW_DATA_URI_IMAGES") {
		rule.AllowDataURIImages = sec.Key("ALLOW_DATA_URI_IMAGES").MustBool(false)
		ok = true
	}

	if sec.HasKey("ELEMENT") || sec.HasKey("ALLOW_ATTR") {
		rule.Element = sec.Key("ELEMENT").Value()
		rule.AllowAttr = sec.Key("ALLOW_ATTR").Value()

		if rule.Element == "" || rule.AllowAttr == "" {
			log.Error("Missing required values from markup.%s. Must have ELEMENT and ALLOW_ATTR defined!", name)
			return rule, false
		}

		regexpStr := sec.Key("REGEXP").Value()
		if regexpStr != "" {
			// Validate when parsing the config that this is a valid regular
			// expression. Then we can use regexp.MustCompile(...) later.
			compiled, err := regexp.Compile(regexpStr)
			if err != nil {
				log.Error("In markup.%s: REGEXP (%s) failed to compile: %v", name, regexpStr, err)
				return rule, false
			}

			rule.Regexp = compiled
		}

		ok = true
	}

	if !ok {
		log.Error("Missing required keys from markup.%s. Must have ELEMENT and ALLOW_ATTR or ALLOW_DATA_URI_IMAGES defined!", name)
		return rule, false
	}

	return rule, true
}

func newMarkupRenderer(name string, sec *ini.Section) {
	extensionReg := regexp.MustCompile(`\.\w`)

	extensions := sec.Key("FILE_EXTENSIONS").Strings(",")
	exts := make([]string, 0, len(extensions))
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

	if sec.HasKey("DISABLE_SANITIZER") {
		log.Error("Deprecated setting `[markup.*]` `DISABLE_SANITIZER` present. This fallback will be removed in v1.18.0")
	}

	renderContentMode := sec.Key("RENDER_CONTENT_MODE").MustString(RenderContentModeSanitized)
	if !sec.HasKey("RENDER_CONTENT_MODE") && sec.Key("DISABLE_SANITIZER").MustBool(false) {
		renderContentMode = RenderContentModeNoSanitizer // if only the legacy DISABLE_SANITIZER exists, use it
	}

	if renderContentMode != RenderContentModeSanitized &&
		renderContentMode != RenderContentModeNoSanitizer &&
		renderContentMode != RenderContentModeIframe {
		log.Error("invalid RENDER_CONTENT_MODE: %q, default to %q", renderContentMode, RenderContentModeSanitized)
		renderContentMode = RenderContentModeSanitized
	}

	ExternalMarkupRenderers = append(ExternalMarkupRenderers, &MarkupRenderer{
		Enabled:                    sec.Key("ENABLED").MustBool(false),
		MarkupName:                 name,
		FileExtensions:             exts,
		Command:                    command,
		IsInputFile:                sec.Key("IS_INPUT_FILE").MustBool(false),
		NeedPostProcess:            sec.Key("NEED_POSTPROCESS").MustBool(true),
		RenderContentMode:          renderContentMode,
		RenderContentIframeSandbox: sec.Key("RENDER_CONTENT_IFRAME_SANDBOX").MustString("allow-scripts"),
		RenderContentExternalCSP:   sec.Key("RENDER_CONTENT_EXTERNAL_CSP").MustString("frame-src 'self'; sandbox allow-scripts"),
	})
}
