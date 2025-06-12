// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
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

type MarkdownRenderOptions struct {
	NewLineHardBreak  bool
	ShortIssuePattern bool // Actually it is a "markup" option because it is used in "post processor"
}

type MarkdownMathCodeBlockOptions struct {
	ParseInlineDollar        bool
	ParseInlineParentheses   bool
	ParseBlockDollar         bool
	ParseBlockSquareBrackets bool
}

// Markdown settings
var Markdown = struct {
	RenderOptionsComment  MarkdownRenderOptions `ini:"-"`
	RenderOptionsWiki     MarkdownRenderOptions `ini:"-"`
	RenderOptionsRepoFile MarkdownRenderOptions `ini:"-"`

	CustomURLSchemes []string `ini:"CUSTOM_URL_SCHEMES"` // Actually it is a "markup" option because it is used in "post processor"
	FileExtensions   []string

	EnableMath             bool
	MathCodeBlockDetection []string
	MathCodeBlockOptions   MarkdownMathCodeBlockOptions `ini:"-"`
}{
	FileExtensions: strings.Split(".md,.markdown,.mdown,.mkd,.livemd", ","),
	EnableMath:     true,
}

// MarkupRenderer defines the external parser configured in ini
type MarkupRenderer struct {
	Enabled              bool
	MarkupName           string
	Command              string
	FileExtensions       []string
	IsInputFile          bool
	NeedPostProcess      bool
	MarkupSanitizerRules []MarkupSanitizerRule
	RenderContentMode    string
}

// MarkupSanitizerRule defines the policy for whitelisting attributes on
// certain elements.
type MarkupSanitizerRule struct {
	Element            string
	AllowAttr          string
	Regexp             string
	AllowDataURIImages bool
}

func loadMarkupFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "markdown", &Markdown)
	const none = "none"

	const renderOptionShortIssuePattern = "short-issue-pattern"
	const renderOptionNewLineHardBreak = "new-line-hard-break"
	cfgMarkdown := rootCfg.Section("markdown")
	parseMarkdownRenderOptions := func(key string, defaults []string) (ret MarkdownRenderOptions) {
		options := cfgMarkdown.Key(key).Strings(",")
		options = util.IfEmpty(options, defaults)
		for _, opt := range options {
			switch opt {
			case renderOptionShortIssuePattern:
				ret.ShortIssuePattern = true
			case renderOptionNewLineHardBreak:
				ret.NewLineHardBreak = true
			case none:
				ret = MarkdownRenderOptions{}
			case "":
			default:
				log.Error("Unknown markdown render option in %s: %s", key, opt)
			}
		}
		return ret
	}
	Markdown.RenderOptionsComment = parseMarkdownRenderOptions("RENDER_OPTIONS_COMMENT", []string{renderOptionShortIssuePattern, renderOptionNewLineHardBreak})
	Markdown.RenderOptionsWiki = parseMarkdownRenderOptions("RENDER_OPTIONS_WIKI", []string{renderOptionShortIssuePattern})
	Markdown.RenderOptionsRepoFile = parseMarkdownRenderOptions("RENDER_OPTIONS_REPO_FILE", nil)

	const mathCodeInlineDollar = "inline-dollar"
	const mathCodeInlineParentheses = "inline-parentheses"
	const mathCodeBlockDollar = "block-dollar"
	const mathCodeBlockSquareBrackets = "block-square-brackets"
	Markdown.MathCodeBlockDetection = util.IfEmpty(Markdown.MathCodeBlockDetection, []string{mathCodeInlineDollar, mathCodeBlockDollar})
	Markdown.MathCodeBlockOptions = MarkdownMathCodeBlockOptions{}
	for _, s := range Markdown.MathCodeBlockDetection {
		switch s {
		case mathCodeInlineDollar:
			Markdown.MathCodeBlockOptions.ParseInlineDollar = true
		case mathCodeInlineParentheses:
			Markdown.MathCodeBlockOptions.ParseInlineParentheses = true
		case mathCodeBlockDollar:
			Markdown.MathCodeBlockOptions.ParseBlockDollar = true
		case mathCodeBlockSquareBrackets:
			Markdown.MathCodeBlockOptions.ParseBlockSquareBrackets = true
		case none:
			Markdown.MathCodeBlockOptions = MarkdownMathCodeBlockOptions{}
		case "":
		default:
			log.Error("Unknown math code block detection option: %s", s)
		}
	}

	MermaidMaxSourceCharacters = rootCfg.Section("markup").Key("MERMAID_MAX_SOURCE_CHARACTERS").MustInt(50000)
	ExternalMarkupRenderers = make([]*MarkupRenderer, 0, 10)
	ExternalSanitizerRules = make([]MarkupSanitizerRule, 0, 10)

	for _, sec := range rootCfg.Section("markup").ChildSections() {
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

func newMarkupSanitizer(name string, sec ConfigSection) {
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

func createMarkupSanitizerRule(name string, sec ConfigSection) (MarkupSanitizerRule, bool) {
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
			hasPrefix := strings.HasPrefix(regexpStr, "^")
			hasSuffix := strings.HasSuffix(regexpStr, "$")
			if !hasPrefix || !hasSuffix {
				log.Error("In markup.%s: REGEXP must start with ^ and end with $ to be strict", name)
				// to avoid breaking existing user configurations and satisfy the strict requirement in addSanitizerRules
				if !hasPrefix {
					regexpStr = "^.*" + regexpStr
				}
				if !hasSuffix {
					regexpStr += ".*$"
				}
			}
			_, err := regexp.Compile(regexpStr)
			if err != nil {
				log.Error("In markup.%s: REGEXP (%s) failed to compile: %v", name, regexpStr, err)
				return rule, false
			}
			rule.Regexp = regexpStr
		}

		ok = true
	}

	if !ok {
		log.Error("Missing required keys from markup.%s. Must have ELEMENT and ALLOW_ATTR or ALLOW_DATA_URI_IMAGES defined!", name)
		return rule, false
	}

	return rule, true
}

func newMarkupRenderer(name string, sec ConfigSection) {
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
		Enabled:           sec.Key("ENABLED").MustBool(false),
		MarkupName:        name,
		FileExtensions:    exts,
		Command:           command,
		IsInputFile:       sec.Key("IS_INPUT_FILE").MustBool(false),
		NeedPostProcess:   sec.Key("NEED_POSTPROCESS").MustBool(true),
		RenderContentMode: renderContentMode,
	})
}
