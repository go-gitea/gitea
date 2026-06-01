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

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/renderhelper"
	"gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/emoji"
	"gitea.dev/modules/git"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/svg"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
	"gitea.dev/services/webtheme"
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
	// wrap "`…`" in <code> before post-processing so code-span content stays literal, like comment bodies
	htmlWithCode := renderCodeBlock(template.HTML(template.HTMLEscapeString(text)))
	renderedText, err := markup.PostProcessIssueTitle(renderhelper.NewRenderContextRepoComment(ut.ctx, repo), string(htmlWithCode))
	if err != nil {
		log.Error("PostProcessIssueTitle: %v", err)
		return ""
	}
	return template.HTML(renderedText)
}

// RenderIssueSimpleTitle only renders with emoji and inline code block
func (ut *RenderUtils) RenderIssueSimpleTitle(text string) template.HTML {
	// see RenderIssueTitle: wrap code spans before processing emoji
	htmlWithCode := renderCodeBlock(template.HTML(template.HTMLEscapeString(text)))
	renderedText, err := markup.PostProcessEmoji(markup.NewRenderContext(ut.ctx), string(htmlWithCode))
	if err != nil {
		log.Error("RenderIssueSimpleTitle: %v", err)
		return ""
	}
	return template.HTML(renderedText)
}

func (ut *RenderUtils) RenderLabel(label *issues_model.Label) template.HTML {
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
		return htmlutil.HTMLFormat(`<span class="ui label %s" style="color: %s !important; background-color: %s !important;" data-tooltip-content title="%s"><span class="gt-ellipsis">%s</span></span>`,
			extraCSSClasses, textColor, label.Color, descriptionText, ut.RenderEmoji(label.Name))
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
		return htmlutil.HTMLFormat(`<span class="ui label %s scope-parent" data-tooltip-content title="%s">`+
			`<div class="ui label scope-left" style="color: %s !important; background-color: %s !important">%s</div>`+
			`<div class="ui label scope-middle" style="color: %s !important; background-color: %s !important">%s</div>`+
			`<div class="ui label scope-right">%d</div>`+
			`</span>`,
			extraCSSClasses, descriptionText,
			textColor, scopeColor, scopeHTML,
			textColor, itemColor, itemHTML,
			label.ExclusiveOrder)
	}

	// <scope> | <label>
	return htmlutil.HTMLFormat(`<span class="ui label %s scope-parent" data-tooltip-content title="%s">`+
		`<div class="ui label scope-left" style="color: %s !important; background-color: %s !important">%s</div>`+
		`<div class="ui label scope-right" style="color: %s !important; background-color: %s !important">%s</div>`+
		`</span>`,
		extraCSSClasses, descriptionText,
		textColor, scopeColor, scopeHTML,
		textColor, itemColor, itemHTML)
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

// RenderPackageMarkdown renders package page Markdown so relative links resolve against the
// linked repository's default branch instead of the site root, falling back to plain rendering
// when there is no linked repository. pkgTreePath optionally roots links in a subdirectory
// (e.g. npm's repository.directory for monorepo packages).
func (ut *RenderUtils) RenderPackageMarkdown(input string, linkedRepo *repo.Repository, pkgTreePath ...string) template.HTML {
	if linkedRepo == nil {
		return `<div class="markup markdown">` + ut.MarkdownToHtml(input) + `</div>`
	}
	rctx := renderhelper.NewRenderContextRepoFile(ut.ctx, linkedRepo, renderhelper.RepoFileOptions{
		CurrentRefSubURL: git.RefNameFromBranch(linkedRepo.DefaultBranch).RefWebLinkPath(),
		CurrentTreePath:  util.OptionalArg(pkgTreePath),
	})
	output, err := markdown.RenderString(rctx, input)
	if err != nil {
		log.Error("RenderString: %v", err)
	}
	return `<div class="markup markdown">` + output + `</div>`
}

func (ut *RenderUtils) RenderLabels(labels []*issues_model.Label, repoLink string, issue *issues_model.Issue) template.HTML {
	isPullRequest := issue != nil && issue.IsPull
	baseLink := fmt.Sprintf("%s/%s", repoLink, util.Iif(isPullRequest, "pulls", "issues"))
	var htmlCode htmlutil.HTMLBuilder
	htmlCode.WriteHTML(`<span class="labels-list">`)
	for _, label := range labels {
		// Protect against nil value in labels - shouldn't happen but would cause a panic if so
		if label == nil {
			continue
		}
		htmlCode.WriteFormat(`<a class="item" href="%s?labels=%d">`, baseLink, label.ID)
		htmlCode.WriteHTML(ut.RenderLabel(label))
		htmlCode.WriteHTML("</a>")
	}
	htmlCode.WriteHTML("</span>")
	return htmlCode.HTMLString()
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

// commitsSearchBase builds the repo+ref scoped commits URL prefix for author-search links,
// or "" when there is no single-ref context (callers then fall back to profile/mailto).
func commitsSearchBase(repoLink, refSubURL string) string {
	if repoLink == "" || refSubURL == "" {
		return ""
	}
	return repoLink + "/commits/" + refSubURL
}

// commitsSearchURL appends an author-search query to searchBase, or "" when there is no base or
// the value contains whitespace (the search parser splits on it).
func commitsSearchURL(searchBase, value string) template.URL {
	if searchBase == "" || value == "" || strings.ContainsAny(value, " \t\r\n") {
		return ""
	}
	return template.URL(searchBase + "/search?q=" + url.QueryEscape("author:"+value))
}

func stackUserSearchEmail(su *user_model.AvatarStackUser) string {
	if su.Sig != nil && su.Sig.Email != "" {
		return su.Sig.Email
	}
	if su.GiteaUser != nil {
		return su.GiteaUser.Email
	}
	return ""
}

func (ut *RenderUtils) stackUserHref(searchBase string, su *user_model.AvatarStackUser) template.URL {
	var fallback string
	switch {
	case su.GiteaUser != nil:
		fallback = su.GiteaUser.HomeLink()
	case su.Sig != nil && su.Sig.Email != "":
		fallback = "mailto:" + su.Sig.Email
	default:
		return ""
	}
	if href := commitsSearchURL(searchBase, stackUserSearchEmail(su)); href != "" {
		return href
	}
	return template.URL(fallback)
}

func (ut *RenderUtils) stackUserAvatar(su *user_model.AvatarStackUser) template.HTML {
	au := NewAvatarUtils(ut.ctx)
	if su.GiteaUser != nil {
		return au.Avatar(su.GiteaUser, 20)
	}
	return au.AvatarByEmail(su.Sig.Email, su.Sig.Name, 20)
}

func stackUserName(su *user_model.AvatarStackUser) string {
	if su.GiteaUser != nil {
		return su.GiteaUser.GetDisplayName()
	}
	return su.Sig.Name
}

// AvatarStack renders overlapping avatars for the stack participants. It emits children in reverse
// so CSS `flex-direction: row-reverse` places the primary (Users[0]) leftmost and last-painted (on top).
func (ut *RenderUtils) AvatarStack(data *user_model.AvatarStackData, repoLink, refSubURL string) template.HTML {
	return ut.avatarStackHTML(data, commitsSearchBase(repoLink, refSubURL))
}

func (ut *RenderUtils) avatarStackHTML(data *user_model.AvatarStackData, searchBase string) template.HTML {
	if data == nil || len(data.Users) == 0 {
		return ""
	}
	const maxVisible = 10
	visible := data.Users
	overflow := 0
	if len(visible) > maxVisible {
		overflow = len(visible) - maxVisible
		visible = visible[:maxVisible]
	}

	var b htmlutil.HTMLBuilder
	b.WriteHTML(`<span class="avatar-stack">`)
	if overflow > 0 {
		b.WriteFormat(`<span class="avatar-stack-overflow-chip tw-text-xs" aria-label="+%d more">+%d</span>`, overflow, overflow)
	}
	for _, su := range slices.Backward(visible) {
		ut.writeAvatarStackItem(&b, searchBase, su)
	}
	b.WriteHTML(`</span>`)
	return b.HTMLString()
}

func (ut *RenderUtils) writeAvatarStackItem(b *htmlutil.HTMLBuilder, searchBase string, su *user_model.AvatarStackUser) {
	avatar := ut.stackUserAvatar(su)
	if href := ut.stackUserHref(searchBase, su); href != "" {
		b.WriteFormat(`<a href="%s">%s</a>`, href, avatar)
	} else {
		b.WriteFormat(`<span>%s</span>`, avatar)
	}
}

// AvatarStackWithNames renders the avatar stack plus a label: `name` / `a and b` / `N people` (opens popup).
func (ut *RenderUtils) AvatarStackWithNames(data *user_model.AvatarStackData, repoLink, refSubURL string) template.HTML {
	if data == nil || len(data.Users) == 0 {
		return ""
	}
	searchBase := commitsSearchBase(repoLink, refSubURL)
	locale := ut.ctx.Value(translation.ContextKey).(translation.Locale)
	users := data.Users

	var b htmlutil.HTMLBuilder
	b.WriteHTML(`<span class="avatar-stack-names">`)
	b.WriteHTML(ut.avatarStackHTML(data, searchBase))

	switch len(users) {
	case 1:
		b.WriteHTML(ut.stackUserNameLink(searchBase, users[0]))
	case 2:
		b.WriteHTML(ut.stackUserNameLink(searchBase, users[0]))
		b.WriteFormat(`<span>%s</span>`, locale.Tr("repo.commits.avatar_stack_and"))
		b.WriteHTML(ut.stackUserNameLink(searchBase, users[1]))
	default:
		b.WriteFormat(`<button type="button" class="avatar-stack-popup-trigger" data-global-init="initAvatarStackPopup">%s</button>`,
			locale.Tr("repo.commits.avatar_stack_people", len(users)))
		b.WriteHTML(`<div class="tippy-target"><div class="avatar-stack-popup">`)
		for _, su := range users {
			b.WriteHTML(ut.stackUserPopupRow(searchBase, su))
		}
		b.WriteHTML(`</div></div>`)
	}

	b.WriteHTML(`</span>`)
	return b.HTMLString()
}

// stackUserNameLink prefers (in order): commits-by-author search, `GetShortDisplayNameLinkHTML` (keeps alt-name tooltip), `mailto:`, bare name.
func (ut *RenderUtils) stackUserNameLink(searchBase string, su *user_model.AvatarStackUser) template.HTML {
	if href := commitsSearchURL(searchBase, stackUserSearchEmail(su)); href != "" {
		return htmlutil.HTMLFormat(`<a class="muted" href="%s">%s</a>`, href, stackUserName(su))
	}
	if su.GiteaUser != nil {
		return su.GiteaUser.GetShortDisplayNameLinkHTML()
	}
	if su.Sig == nil {
		return ""
	}
	if su.Sig.Email != "" {
		return htmlutil.HTMLFormat(`<a class="muted" href="mailto:%s">%s</a>`, su.Sig.Email, su.Sig.Name)
	}
	return template.HTML(template.HTMLEscapeString(su.Sig.Name))
}

func (ut *RenderUtils) stackUserPopupRow(searchBase string, su *user_model.AvatarStackUser) template.HTML {
	avatar := ut.stackUserAvatar(su)
	name := stackUserName(su)
	if href := ut.stackUserHref(searchBase, su); href != "" {
		return htmlutil.HTMLFormat(`<a class="silenced flex-text-block" href="%s">%s<span>%s</span></a>`, href, avatar, name)
	}
	return htmlutil.HTMLFormat(`<span class="flex-text-block">%s<span>%s</span></span>`, avatar, name)
}
