// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"encoding/hex"
	"fmt"
	"html/template"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/renderhelper"
	"code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/webtheme"
)

type RenderUtils struct {
	ctx reqctx.RequestContext
}

func NewRenderUtils(ctx reqctx.RequestContext) *RenderUtils {
	return &RenderUtils{ctx: ctx}
}

// RenderCommitMessage renders commit message with XSS-safe and special links.
func (ut *RenderUtils) RenderCommitMessage(msg string, repo *repo.Repository) template.HTML {
	cleanMsg := template.HTML(template.HTMLEscapeString(msg))
	// we can safely assume that it will not return any error, since there shouldn't be any special HTML.
	// "repo" can be nil when rendering commit messages for deleted repositories in a user's dashboard feed.
	fullMessage, err := markup.PostProcessCommitMessage(renderhelper.NewRenderContextRepoComment(ut.ctx, repo), cleanMsg)
	if err != nil {
		log.Error("PostProcessCommitMessage: %v", err)
		return ""
	}
	msgLines := strings.Split(strings.TrimSpace(string(fullMessage)), "\n")
	if len(msgLines) == 0 {
		return ""
	}
	return renderCodeBlock(template.HTML(msgLines[0]))
}

// RenderCommitMessageLinkSubject renders commit message as a XSS-safe link to
// the provided default url, handling for special links without email to links.
func (ut *RenderUtils) RenderCommitMessageLinkSubject(msg, urlDefault string, repo *repo.Repository) template.HTML {
	msgLine := strings.TrimLeftFunc(msg, unicode.IsSpace)
	lineEnd := strings.IndexByte(msgLine, '\n')
	if lineEnd > 0 {
		msgLine = msgLine[:lineEnd]
	}
	msgLine = strings.TrimRightFunc(msgLine, unicode.IsSpace)
	if len(msgLine) == 0 {
		return ""
	}

	// we can safely assume that it will not return any error, since there shouldn't be any special HTML.
	renderedMessage, err := markup.PostProcessCommitMessageSubject(renderhelper.NewRenderContextRepoComment(ut.ctx, repo), urlDefault, template.HTMLEscapeString(msgLine))
	if err != nil {
		log.Error("PostProcessCommitMessageSubject: %v", err)
		return ""
	}
	return renderCodeBlock(template.HTML(renderedMessage))
}

// RenderCommitBody extracts the body of a commit message without its title.
func (ut *RenderUtils) RenderCommitBody(msg string, repo *repo.Repository) template.HTML {
	_, body, _ := strings.Cut(strings.TrimSpace(msg), "\n")
	body = strings.TrimFunc(body, unicode.IsSpace)
	if body == "" {
		return ""
	}

	rctx := renderhelper.NewRenderContextRepoComment(ut.ctx, repo)
	htmlContent := template.HTML(template.HTMLEscapeString(body))
	renderedMessage, err := markup.PostProcessCommitMessage(rctx, htmlContent)
	if err != nil {
		log.Error("PostProcessCommitMessage: %v", err)
		return ""
	}
	return renderedMessage
}

// Match text that is between back ticks.
var codeMatcher = regexp.MustCompile("`([^`]+)`")

// renderCodeBlock renders "`…`" as highlighted "<code>" block, intended for issue and PR titles
func renderCodeBlock(htmlEscapedTextToRender template.HTML) template.HTML {
	htmlWithCodeTags := codeMatcher.ReplaceAllString(string(htmlEscapedTextToRender), `<code class="inline-code-block">$1</code>`) // replace with HTML <code> tags
	return template.HTML(htmlWithCodeTags)
}

// RenderIssueTitle renders issue/pull title with defined post processors
func (ut *RenderUtils) RenderIssueTitle(text string, repo *repo.Repository) template.HTML {
	renderedText, err := markup.PostProcessIssueTitle(renderhelper.NewRenderContextRepoComment(ut.ctx, repo), template.HTMLEscapeString(text))
	if err != nil {
		log.Error("PostProcessIssueTitle: %v", err)
		return ""
	}
	return renderCodeBlock(template.HTML(renderedText))
}

// RenderIssueSimpleTitle only renders with emoji and inline code block
func (ut *RenderUtils) RenderIssueSimpleTitle(text string) template.HTML {
	ret := ut.RenderEmoji(text)
	ret = renderCodeBlock(ret)
	return ret
}

func (ut *RenderUtils) RenderLabelWithLink(label *issues_model.Label, link any) template.HTML {
	var attrHref template.HTML
	switch link.(type) {
	case template.URL, string:
		attrHref = htmlutil.HTMLFormat(`href="%s"`, link)
	default:
		panic(fmt.Sprintf("unexpected type %T for link", link))
	}
	return ut.renderLabelWithTag(label, "a", attrHref)
}

func (ut *RenderUtils) RenderLabel(label *issues_model.Label) template.HTML {
	return ut.renderLabelWithTag(label, "span", "")
}

// RenderLabel renders a label
func (ut *RenderUtils) renderLabelWithTag(label *issues_model.Label, tagName, tagAttrs template.HTML) template.HTML {
	locale := ut.ctx.Value(translation.ContextKey).(translation.Locale)
	var extraCSSClasses string
	textColor := util.ContrastColor(label.Color)
	labelScope := label.ExclusiveScope()
	descriptionText := emoji.ReplaceAliases(label.Description)

	if label.IsArchived() {
		extraCSSClasses = "archived-label"
		descriptionText = fmt.Sprintf("(%s) %s", locale.TrString("archived"), descriptionText)
	}

	if labelScope == "" {
		// Regular label
		return htmlutil.HTMLFormat(`<%s %s class="ui label %s" style="color: %s !important; background-color: %s !important;" data-tooltip-content title="%s"><span class="gt-ellipsis">%s</span></%s>`,
			tagName, tagAttrs, extraCSSClasses, textColor, label.Color, descriptionText, ut.RenderEmoji(label.Name), tagName)
	}

	// Scoped label
	scopeHTML := ut.RenderEmoji(labelScope)
	itemHTML := ut.RenderEmoji(label.Name[len(labelScope)+1:])

	// Make scope and item background colors slightly darker and lighter respectively.
	// More contrast needed with higher luminance, empirically tweaked.
	luminance := util.GetRelativeLuminance(label.Color)
	contrast := 0.01 + luminance*0.03
	// Ensure we add the same amount of contrast also near 0 and 1.
	darken := contrast + math.Max(luminance+contrast-1.0, 0.0)
	lighten := contrast + math.Max(contrast-luminance, 0.0)
	// Compute the factor to keep RGB values proportional.
	darkenFactor := math.Max(luminance-darken, 0.0) / math.Max(luminance, 1.0/255.0)
	lightenFactor := math.Min(luminance+lighten, 1.0) / math.Max(luminance, 1.0/255.0)

	r, g, b := util.HexToRBGColor(label.Color)
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

	if label.ExclusiveOrder > 0 {
		// <scope> | <label> | <order>
		return htmlutil.HTMLFormat(`<%s %s class="ui label %s scope-parent" data-tooltip-content title="%s">`+
			`<div class="ui label scope-left" style="color: %s !important; background-color: %s !important">%s</div>`+
			`<div class="ui label scope-middle" style="color: %s !important; background-color: %s !important">%s</div>`+
			`<div class="ui label scope-right">%d</div>`+
			`</%s>`,
			tagName, tagAttrs,
			extraCSSClasses, descriptionText,
			textColor, scopeColor, scopeHTML,
			textColor, itemColor, itemHTML,
			label.ExclusiveOrder,
			tagName)
	}

	// <scope> | <label>
	return htmlutil.HTMLFormat(`<%s %s class="ui label %s scope-parent" data-tooltip-content title="%s">`+
		`<div class="ui label scope-left" style="color: %s !important; background-color: %s !important">%s</div>`+
		`<div class="ui label scope-right" style="color: %s !important; background-color: %s !important">%s</div>`+
		`</%s>`,
		tagName, tagAttrs,
		extraCSSClasses, descriptionText,
		textColor, scopeColor, scopeHTML,
		textColor, itemColor, itemHTML,
		tagName)
}

// RenderEmoji renders html text with emoji post processors
func (ut *RenderUtils) RenderEmoji(text string) template.HTML {
	renderedText, err := markup.PostProcessEmoji(markup.NewRenderContext(ut.ctx), template.HTMLEscapeString(text))
	if err != nil {
		log.Error("RenderEmoji: %v", err)
		return ""
	}
	return template.HTML(renderedText)
}

// reactionToEmoji renders emoji for use in reactions
func reactionToEmoji(reaction string) template.HTML {
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

func (ut *RenderUtils) MarkdownToHtml(input string) template.HTML { //nolint:revive // variable naming triggers on Html, wants HTML
	output, err := markdown.RenderString(markup.NewRenderContext(ut.ctx).WithMetas(markup.ComposeSimpleDocumentMetas()), input)
	if err != nil {
		log.Error("RenderString: %v", err)
	}
	return output
}

func (ut *RenderUtils) RenderLabels(labels []*issues_model.Label, repoLink string, issue *issues_model.Issue) template.HTML {
	isPullRequest := issue != nil && issue.IsPull
	baseLink := fmt.Sprintf("%s/%s", repoLink, util.Iif(isPullRequest, "pulls", "issues"))
	var htmlCode strings.Builder
	htmlCode.WriteString(`<span class="labels-list">`)
	for _, label := range labels {
		// Protect against nil value in labels - shouldn't happen but would cause a panic if so
		if label == nil {
			continue
		}
		link := fmt.Sprintf("%s?labels=%d", baseLink, label.ID)
		htmlCode.WriteString(string(ut.RenderLabelWithLink(label, template.URL(link))))
	}
	htmlCode.WriteString("</span>")
	return template.HTML(htmlCode.String())
}

func (ut *RenderUtils) RenderThemeItem(info *webtheme.ThemeMetaInfo, iconSize int) template.HTML {
	svgName := "octicon-paintbrush"
	switch info.ColorScheme {
	case "dark":
		svgName = "octicon-moon"
	case "light":
		svgName = "octicon-sun"
	case "auto":
		svgName = "gitea-eclipse"
	}
	icon := svg.RenderHTML(svgName, iconSize)
	extraIcon := svg.RenderHTML(info.GetExtraIconName(), iconSize)
	return htmlutil.HTMLFormat(`<div class="theme-menu-item" data-tooltip-content="%s">%s %s %s</div>`, info.GetDescription(), icon, info.DisplayName, extraIcon)
}

func (ut *RenderUtils) RenderFlashMessage(typ, msg string) template.HTML {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}

	cls := typ
	// legacy logic: "negative" for error, "positive" for success
	switch cls {
	case "error":
		cls = "negative"
	case "success":
		cls = "positive"
	}

	var msgContent template.HTML
	if strings.Contains(msg, "</pre>") || strings.Contains(msg, "</details>") || strings.Contains(msg, "</ul>") || strings.Contains(msg, "</div>") {
		// If the message contains some known "block" elements, no need to do more alignment or line-break processing, just sanitize it directly.
		msgContent = sanitizeHTML(msg)
	} else if !strings.Contains(msg, "\n") {
		// If the message is a single line, center-align it by wrapping it
		msgContent = htmlutil.HTMLFormat(`<div class="tw-text-center">%s</div>`, sanitizeHTML(msg))
	} else {
		// For a multi-line message, preserve line breaks, and left-align it.
		msgContent = htmlutil.HTMLFormat(`%s`, sanitizeHTML(strings.ReplaceAll(msg, "\n", "<br>")))
	}
	return htmlutil.HTMLFormat(`<div class="ui %s message flash-message flash-%s">%s</div>`, cls, typ, msgContent)
}

func (ut *RenderUtils) RenderUnicodeEscapeToggleButton(escapeStatus *charset.EscapeStatus) template.HTML {
	if escapeStatus == nil || !escapeStatus.Escaped {
		return ""
	}
	locale := ut.ctx.Value(translation.ContextKey).(translation.Locale)
	var title template.HTML
	if escapeStatus.HasAmbiguous {
		title += locale.Tr("repo.ambiguous_runes_line")
	} else if escapeStatus.HasInvisible {
		title += locale.Tr("repo.invisible_runes_line")
	}
	return htmlutil.HTMLFormat(`<button type="button" class="toggle-escape-button btn interact-bg" title="%s"></button>`, title)
}

func (ut *RenderUtils) RenderUnicodeEscapeToggleTd(combined, escapeStatus *charset.EscapeStatus) template.HTML {
	if combined == nil || !combined.Escaped {
		return ""
	}
	return `<td class="lines-escape">` + ut.RenderUnicodeEscapeToggleButton(escapeStatus) + `</td>`
}

// commitAuthorSearchURL rejects whitespace because the search parser splits on it.
func (ut *RenderUtils) commitAuthorSearchURL(value string) template.URL {
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return ""
	}
	data := ut.ctx.GetData()
	repoLink, _ := data["RepoLink"].(string)
	refSubURL, _ := data["RefTypeNameSubURL"].(string)
	if repoLink == "" || refSubURL == "" {
		return ""
	}
	return template.URL(repoLink + "/commits/" + refSubURL + "/search?q=" + url.QueryEscape("author:"+value))
}

func authorSearchEmail(u *user_model.User, sig *git.Signature) string {
	if sig != nil && sig.Email != "" {
		return sig.Email
	}
	if u != nil {
		return u.Email
	}
	return ""
}

func (ut *RenderUtils) authorHref(u *user_model.User, sig *git.Signature) template.URL {
	var fallback string
	switch {
	case u != nil:
		fallback = u.HomeLink()
	case sig != nil && sig.Email != "":
		fallback = "mailto:" + sig.Email
	default:
		return ""
	}
	if href := ut.commitAuthorSearchURL(authorSearchEmail(u, sig)); href != "" {
		return href
	}
	return template.URL(fallback)
}

func (ut *RenderUtils) authorAvatar(u *user_model.User, sig *git.Signature) template.HTML {
	au := NewAvatarUtils(ut.ctx)
	if u != nil {
		return au.Avatar(u, 20)
	}
	return au.AvatarByEmail(sig.Email, sig.Name, 20)
}

func authorDisplayName(u *user_model.User, sig *git.Signature) string {
	if u != nil {
		return u.GetDisplayName()
	}
	return sig.Name
}

// AvatarStack emits children in reverse paint order so CSS `flex-direction: row-reverse` places the author leftmost and last-painted (on top).
func (ut *RenderUtils) AvatarStack(authorUser *user_model.User, authorSig *git.Signature, coAuthors []*user_model.CoAuthorUser) template.HTML {
	if authorUser == nil && authorSig == nil {
		return ""
	}

	const maxCo = 9
	visibleCo := coAuthors
	overflow := 0
	if len(coAuthors) > maxCo {
		visibleCo = coAuthors[:maxCo]
		overflow = len(coAuthors) - maxCo
	}

	var b htmlutil.HTMLBuilder
	b.WriteHTML(`<span class="avatar-stack">`)

	if overflow > 0 {
		b.WriteFormat(`<span class="avatar-stack-overflow-chip tw-text-xs" aria-label="+%d more">+%d</span>`, overflow, overflow)
	}
	for _, co := range slices.Backward(visibleCo) {
		ut.appendAvatarStackChild(&b, co.GiteaUser, co.TrailerSignature)
	}
	ut.appendAvatarStackChild(&b, authorUser, authorSig)

	b.WriteHTML(`</span>`)
	return b.HTMLString()
}

func (ut *RenderUtils) appendAvatarStackChild(b *htmlutil.HTMLBuilder, u *user_model.User, sig *git.Signature) {
	avatar := ut.authorAvatar(u, sig)
	if href := ut.authorHref(u, sig); href != "" {
		b.WriteFormat(`<a href="%s">%s</a>`, href, avatar)
	} else {
		b.WriteFormat(`<span>%s</span>`, avatar)
	}
}

// CoAuthorAvatars renders the avatar stack plus a label: `name` / `a and b` / `N people` (opens popup).
func (ut *RenderUtils) CoAuthorAvatars(data *user_model.CoAuthorAvatarData) template.HTML {
	if data == nil {
		return ""
	}
	authorUser, authorSig, coAuthors := data.AuthorUser, data.AuthorSig, data.CoAuthors
	locale := ut.ctx.Value(translation.ContextKey).(translation.Locale)

	var b htmlutil.HTMLBuilder
	b.WriteHTML(`<span class="author-wrapper">`)
	b.WriteHTML(ut.AvatarStack(authorUser, authorSig, coAuthors))

	switch len(coAuthors) {
	case 0:
		b.WriteHTML(ut.authorNameLinkHTML(authorUser, authorSig))
	case 1:
		b.WriteHTML(ut.authorNameLinkHTML(authorUser, authorSig))
		b.WriteFormat(`<span>%s</span>`, locale.Tr("repo.commits.coauthor_and"))
		b.WriteHTML(ut.authorNameLinkHTML(coAuthors[0].GiteaUser, coAuthors[0].TrailerSignature))
	default:
		b.WriteFormat(`<button type="button" class="authors-popup-trigger" data-global-init="initAuthorsPopup">%s</button>`,
			locale.Tr("repo.commits.coauthor_people", len(coAuthors)+1))
		b.WriteHTML(`<div class="tippy-target"><div class="authors-popup">`)
		b.WriteHTML(ut.participantRowHTML(authorUser, authorSig))
		for _, co := range coAuthors {
			b.WriteHTML(ut.participantRowHTML(co.GiteaUser, co.TrailerSignature))
		}
		b.WriteHTML(`</div></div>`)
	}

	b.WriteHTML(`</span>`)
	return b.HTMLString()
}

// authorNameLinkHTML prefers (in order): commits-by-author search, `GetShortDisplayNameLinkHTML` (keeps alt-name tooltip), `mailto:`, bare name.
func (ut *RenderUtils) authorNameLinkHTML(u *user_model.User, sig *git.Signature) template.HTML {
	if href := ut.commitAuthorSearchURL(authorSearchEmail(u, sig)); href != "" {
		return htmlutil.HTMLFormat(`<a class="muted" href="%s">%s</a>`, href, authorDisplayName(u, sig))
	}
	if u != nil {
		return u.GetShortDisplayNameLinkHTML()
	}
	if sig == nil {
		return ""
	}
	if sig.Email != "" {
		return htmlutil.HTMLFormat(`<a class="muted" href="mailto:%s">%s</a>`, sig.Email, sig.Name)
	}
	return template.HTML(template.HTMLEscapeString(sig.Name))
}

func (ut *RenderUtils) participantRowHTML(u *user_model.User, sig *git.Signature) template.HTML {
	avatar := ut.authorAvatar(u, sig)
	name := authorDisplayName(u, sig)
	if href := ut.authorHref(u, sig); href != "" {
		return htmlutil.HTMLFormat(`<a class="silenced flex-text-block" href="%s">%s<span>%s</span></a>`, href, avatar, name)
	}
	return htmlutil.HTMLFormat(`<span class="flex-text-block">%s<span>%s</span></span>`, avatar, name)
}
