// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"encoding/hex"
	"fmt"
	"html/template"
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// RenderCommitMessage renders commit message with XSS-safe and special links.
func RenderCommitMessage(ctx context.Context, msg, urlPrefix string, metas map[string]string) template.HTML {
	return RenderCommitMessageLink(ctx, msg, urlPrefix, "", metas)
}

// RenderCommitMessageLink renders commit message as a XXS-safe link to the provided
// default url, handling for special links.
func RenderCommitMessageLink(ctx context.Context, msg, urlPrefix, urlDefault string, metas map[string]string) template.HTML {
	cleanMsg := template.HTMLEscapeString(msg)
	// we can safely assume that it will not return any error, since there
	// shouldn't be any special HTML.
	fullMessage, err := markup.RenderCommitMessage(&markup.RenderContext{
		Ctx:         ctx,
		URLPrefix:   urlPrefix,
		DefaultLink: urlDefault,
		Metas:       metas,
	}, cleanMsg)
	if err != nil {
		log.Error("RenderCommitMessage: %v", err)
		return ""
	}
	msgLines := strings.Split(strings.TrimSpace(fullMessage), "\n")
	if len(msgLines) == 0 {
		return template.HTML("")
	}
	return template.HTML(msgLines[0])
}

// RenderCommitMessageLinkSubject renders commit message as a XXS-safe link to
// the provided default url, handling for special links without email to links.
func RenderCommitMessageLinkSubject(ctx context.Context, msg, urlPrefix, urlDefault string, metas map[string]string) template.HTML {
	msgLine := strings.TrimLeftFunc(msg, unicode.IsSpace)
	lineEnd := strings.IndexByte(msgLine, '\n')
	if lineEnd > 0 {
		msgLine = msgLine[:lineEnd]
	}
	msgLine = strings.TrimRightFunc(msgLine, unicode.IsSpace)
	if len(msgLine) == 0 {
		return template.HTML("")
	}

	// we can safely assume that it will not return any error, since there
	// shouldn't be any special HTML.
	renderedMessage, err := markup.RenderCommitMessageSubject(&markup.RenderContext{
		Ctx:         ctx,
		URLPrefix:   urlPrefix,
		DefaultLink: urlDefault,
		Metas:       metas,
	}, template.HTMLEscapeString(msgLine))
	if err != nil {
		log.Error("RenderCommitMessageSubject: %v", err)
		return template.HTML("")
	}
	return template.HTML(renderedMessage)
}

// RenderCommitBody extracts the body of a commit message without its title.
func RenderCommitBody(ctx context.Context, msg, urlPrefix string, metas map[string]string) template.HTML {
	msgLine := strings.TrimRightFunc(msg, unicode.IsSpace)
	lineEnd := strings.IndexByte(msgLine, '\n')
	if lineEnd > 0 {
		msgLine = msgLine[lineEnd+1:]
	} else {
		return template.HTML("")
	}
	msgLine = strings.TrimLeftFunc(msgLine, unicode.IsSpace)
	if len(msgLine) == 0 {
		return template.HTML("")
	}

	renderedMessage, err := markup.RenderCommitMessage(&markup.RenderContext{
		Ctx:       ctx,
		URLPrefix: urlPrefix,
		Metas:     metas,
	}, template.HTMLEscapeString(msgLine))
	if err != nil {
		log.Error("RenderCommitMessage: %v", err)
		return ""
	}
	return template.HTML(renderedMessage)
}

// Match text that is between back ticks.
var codeMatcher = regexp.MustCompile("`([^`]+)`")

// RenderCodeBlock renders "`…`" as highlighted "<code>" block.
// Intended for issue and PR titles, these containers should have styles for "<code>" elements
func RenderCodeBlock(htmlEscapedTextToRender template.HTML) template.HTML {
	htmlWithCodeTags := codeMatcher.ReplaceAllString(string(htmlEscapedTextToRender), "<code>$1</code>") // replace with HTML <code> tags
	return template.HTML(htmlWithCodeTags)
}

// RenderIssueTitle renders issue/pull title with defined post processors
func RenderIssueTitle(ctx context.Context, text, urlPrefix string, metas map[string]string) template.HTML {
	renderedText, err := markup.RenderIssueTitle(&markup.RenderContext{
		Ctx:       ctx,
		URLPrefix: urlPrefix,
		Metas:     metas,
	}, template.HTMLEscapeString(text))
	if err != nil {
		log.Error("RenderIssueTitle: %v", err)
		return template.HTML("")
	}
	return template.HTML(renderedText)
}

// RenderLabel renders a label
func RenderLabel(ctx context.Context, label *issues_model.Label) template.HTML {
	labelScope := label.ExclusiveScope()

	textColor := "#111"
	r, g, b := util.HexToRBGColor(label.Color)
	// Determine if label text should be light or dark to be readable on background color
	if util.UseLightTextOnBackground(r, g, b) {
		textColor = "#eee"
	}

	description := emoji.ReplaceAliases(template.HTMLEscapeString(label.Description))

	if labelScope == "" {
		// Regular label
		s := fmt.Sprintf("<div class='ui label' style='color: %s !important; background-color: %s !important' title='%s'>%s</div>",
			textColor, label.Color, description, RenderEmoji(ctx, label.Name))
		return template.HTML(s)
	}

	// Scoped label
	scopeText := RenderEmoji(ctx, labelScope)
	itemText := RenderEmoji(ctx, label.Name[len(labelScope)+1:])

	// Make scope and item background colors slightly darker and lighter respectively.
	// More contrast needed with higher luminance, empirically tweaked.
	luminance := util.GetLuminance(r, g, b)
	contrast := 0.01 + luminance*0.03
	// Ensure we add the same amount of contrast also near 0 and 1.
	darken := contrast + math.Max(luminance+contrast-1.0, 0.0)
	lighten := contrast + math.Max(contrast-luminance, 0.0)
	// Compute factor to keep RGB values proportional.
	darkenFactor := math.Max(luminance-darken, 0.0) / math.Max(luminance, 1.0/255.0)
	lightenFactor := math.Min(luminance+lighten, 1.0) / math.Max(luminance, 1.0/255.0)

	scopeBytes := []byte{
		uint8(math.Min(math.Round(r*darkenFactor), 255)),
		uint8(math.Min(math.Round(g*darkenFactor), 255)),
		uint8(math.Min(math.Round(b*darkenFactor), 255)),
	}
	itemBytes := []byte{
		uint8(math.Min(math.Round(r*lightenFactor), 255)),
		uint8(math.Min(math.Round(g*lightenFactor), 255)),
		uint8(math.Min(math.Round(b*lightenFactor), 255)),
	}

	itemColor := "#" + hex.EncodeToString(itemBytes)
	scopeColor := "#" + hex.EncodeToString(scopeBytes)

	s := fmt.Sprintf("<span class='ui label scope-parent' title='%s'>"+
		"<div class='ui label scope-left' style='color: %s !important; background-color: %s !important'>%s</div>"+
		"<div class='ui label scope-right' style='color: %s !important; background-color: %s !important''>%s</div>"+
		"</span>",
		description,
		textColor, scopeColor, scopeText,
		textColor, itemColor, itemText)
	return template.HTML(s)
}

// RenderEmoji renders html text with emoji post processors
func RenderEmoji(ctx context.Context, text string) template.HTML {
	renderedText, err := markup.RenderEmoji(&markup.RenderContext{Ctx: ctx},
		template.HTMLEscapeString(text))
	if err != nil {
		log.Error("RenderEmoji: %v", err)
		return template.HTML("")
	}
	return template.HTML(renderedText)
}

// ReactionToEmoji renders emoji for use in reactions
func ReactionToEmoji(reaction string) template.HTML {
	val := emoji.FromCode(reaction)
	if val != nil {
		return template.HTML(val.Emoji)
	}
	val = emoji.FromAlias(reaction)
	if val != nil {
		return template.HTML(val.Emoji)
	}
	return template.HTML(fmt.Sprintf(`<img alt=":%s:" src="%s/assets/img/emoji/%s.png"></img>`, reaction, setting.StaticURLPrefix, url.PathEscape(reaction)))
}

// RenderNote renders the contents of a git-notes file as a commit message.
func RenderNote(ctx context.Context, msg, urlPrefix string, metas map[string]string) template.HTML {
	cleanMsg := template.HTMLEscapeString(msg)
	fullMessage, err := markup.RenderCommitMessage(&markup.RenderContext{
		Ctx:       ctx,
		URLPrefix: urlPrefix,
		Metas:     metas,
	}, cleanMsg)
	if err != nil {
		log.Error("RenderNote: %v", err)
		return ""
	}
	return template.HTML(fullMessage)
}

func RenderMarkdownToHtml(ctx context.Context, input string) template.HTML { //nolint:revive
	output, err := markdown.RenderString(&markup.RenderContext{
		Ctx:       ctx,
		URLPrefix: setting.AppSubURL,
	}, input)
	if err != nil {
		log.Error("RenderString: %v", err)
	}
	return template.HTML(output)
}

func RenderLabels(ctx context.Context, labels []*issues_model.Label, repoLink string) template.HTML {
	htmlCode := `<span class="labels-list">`
	for _, label := range labels {
		// Protect against nil value in labels - shouldn't happen but would cause a panic if so
		if label == nil {
			continue
		}
		htmlCode += fmt.Sprintf("<a href='%s/issues?labels=%d'>%s</a> ",
			repoLink, label.ID, RenderLabel(ctx, label))
	}
	htmlCode += "</span>"
	return template.HTML(htmlCode)
}
