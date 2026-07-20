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

	user_model "gitea.dev/models/gituser"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/renderhelper"
	"gitea.dev/models/repo"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/emoji"
	"gitea.dev/modules/git"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/repository"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/svg"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
	"gitea.dev/services/webtheme"
)

type RenderUtils struct {
	ctx         reqctx.RequestContext
	avatarUtils *AvatarUtils
}

func NewRenderUtils(ctx reqctx.RequestContext) *RenderUtils {
	return &RenderUtils{ctx: ctx, avatarUtils: NewAvatarUtils(ctx)}
}

// RenderCommitMessage renders commit message title (only title)
func (ut *RenderUtils) RenderCommitMessage(msg string, repo *repo.Repository) template.HTML {
	msgLine := strings.TrimSpace(msg)
	msgLine, _, _ = strings.Cut(msgLine, "\n")
	msgLine = strings.TrimSpace(msgLine)
	rendered := markup.PostProcessCommitMessage(renderhelper.NewRenderContextRepoComment(ut.ctx, repo), htmlutil.EscapeString(msgLine))
	return renderCodeBlock(rendered)
}

// RenderCommitMessageLinkSubject renders commit message as a XSS-safe link to
// the provided default url, handling for special links without email to links.
func (ut *RenderUtils) RenderCommitMessageLinkSubject(msg, urlDefault string, repo *repo.Repository) template.HTML {
	msgLine := strings.TrimSpace(msg)
	msgLine, _, _ = strings.Cut(msgLine, "\n")
	msgLine = strings.TrimSpace(msgLine)
	rctx := renderhelper.NewRenderContextRepoComment(ut.ctx, repo)
	rendered := markup.PostProcessCommitMessageSubject(rctx, urlDefault, htmlutil.EscapeString(msgLine))
	return renderCodeBlock(rendered)
}

// RenderCommitBody extracts the body of a commit message without its title.
func (ut *RenderUtils) RenderCommitBody(msg string, repo *repo.Repository) template.HTML {
	_, body, _ := strings.Cut(strings.TrimSpace(msg), "\n")
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	rctx := renderhelper.NewRenderContextRepoComment(ut.ctx, repo)
	renderedMessage := markup.PostProcessCommitMessage(rctx, htmlutil.EscapeString(body))
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
	htmlWithCode := renderCodeBlock(htmlutil.EscapeString(text))
	return markup.PostProcessIssueTitle(renderhelper.NewRenderContextRepoComment(ut.ctx, repo), htmlWithCode)
}

// RenderIssueSimpleTitle only renders with emoji and inline code block
func (ut *RenderUtils) RenderIssueSimpleTitle(text string) template.HTML {
	// see RenderIssueTitle: wrap code spans before processing emoji
	htmlWithCode := renderCodeBlock(htmlutil.EscapeString(text))
	return markup.PostProcessEmoji(markup.NewRenderContext(ut.ctx), htmlWithCode)
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
	return markup.PostProcessEmoji(markup.NewRenderContext(ut.ctx), htmlutil.EscapeString(text))
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

func renderAvatarStackViewEmailLink(data *user_model.AvatarStackData, email string) template.URL {
	if data.SearchByEmailLink != "" && email != "" {
		return template.URL(strings.ReplaceAll(data.SearchByEmailLink, "{email}", url.QueryEscape(email)))
	}
	return ""
}

func (ut *RenderUtils) participantHref(data *user_model.AvatarStackData, participant *user_model.CommitParticipant) template.URL {
	if href := renderAvatarStackViewEmailLink(data, participant.GitIdentity.Email); href != "" {
		return href
	}
	if participant.GiteaUser != nil {
		return template.URL(participant.GiteaUser.HomeLink())
	} else if participant.GitIdentity.Email != "" {
		return template.URL("mailto:" + participant.GitIdentity.Email)
	}
	return ""
}

func (ut *RenderUtils) participantAvatar(participant *user_model.CommitParticipant) template.HTML {
	if participant.GiteaUser != nil {
		return ut.avatarUtils.Avatar(participant.GiteaUser, 20)
	}
	return ut.avatarUtils.AvatarByEmail(participant.GitIdentity.Email, participant.GitIdentity.Name, 20)
}

func participantName(participant *user_model.CommitParticipant) string {
	if participant.GiteaUser != nil {
		return participant.GiteaUser.GetDisplayName()
	}
	return participant.GitIdentity.Name
}

const renderAvatarStackMaxVisible = 10

// AvatarStack renders overlapping avatars for the stack participants. It emits children in reverse
// so CSS `flex-direction: row-reverse` places the primary (Participants[0]) leftmost and last-painted (on top).
func (ut *RenderUtils) AvatarStack(data *user_model.AvatarStackData) template.HTML {
	visible := data.Participants
	overflow := len(visible) - renderAvatarStackMaxVisible
	if overflow > 0 {
		visible = visible[:renderAvatarStackMaxVisible]
	}

	var b htmlutil.HTMLBuilder
	b.WriteHTML(`<span class="avatar-stack">`)
	if overflow > 0 {
		b.WriteFormat(`<span class="avatar-stack-overflow-chip tw-text-xs" aria-label="+%d more">+%d</span>`, overflow, overflow)
	}

	// FIXME: such "backward" breaks a11y like screen readers
	for _, participant := range slices.Backward(visible) {
		ut.writeAvatarStackItem(&b, data, participant)
	}
	b.WriteHTML(`</span>`)
	return b.HTMLString()
}

func (ut *RenderUtils) writeAvatarStackItem(b *htmlutil.HTMLBuilder, data *user_model.AvatarStackData, participant *user_model.CommitParticipant) {
	avatar := ut.participantAvatar(participant)
	if href := ut.participantHref(data, participant); href != "" {
		b.WriteFormat(`<a href="%s">%s</a>`, href, avatar)
	} else {
		b.WriteFormat(`<span>%s</span>`, avatar)
	}
}

func (ut *RenderUtils) AvatarStackPushCommit(pushCommit *repository.PushCommit) template.HTML {
	fakeGitCommit := git.Commit{
		CommitMessage: git.CommitMessage{MessageRaw: pushCommit.Message},
		Author:        &git.Signature{Name: pushCommit.AuthorName, Email: pushCommit.AuthorEmail},
		// there is no way to know the real committer, but the field can't be nil
		Committer: &git.Signature{Name: pushCommit.AuthorName, Email: pushCommit.AuthorEmail},
	}
	data := user_model.BuildAvatarStackData(ut.ctx, fakeGitCommit.AllParticipantIdentities(), nil)
	return ut.AvatarStack(data)
}

// AvatarStackWithNames renders the avatar stack plus a label: `name` / `a and b` / `N people` (opens popup).
func (ut *RenderUtils) AvatarStackWithNames(data *user_model.AvatarStackData) template.HTML {
	locale := ut.ctx.Value(translation.ContextKey).(translation.Locale)
	participants := data.Participants

	var b htmlutil.HTMLBuilder
	b.WriteHTML(`<span class="avatar-stack-names">`)
	b.WriteHTML(ut.AvatarStack(data))

	switch len(participants) {
	case 1:
		b.WriteHTML(ut.participantNameLink(data, participants[0]))
	case 2:
		b.WriteHTML(ut.participantNameLink(data, participants[0]))
		b.WriteFormat(`<span>%s</span>`, locale.Tr("repo.commits.avatar_stack_and"))
		b.WriteHTML(ut.participantNameLink(data, participants[1]))
	default:
		b.WriteFormat(`<button type="button" class="avatar-stack-popup-trigger" data-global-init="initAvatarStackPopup">%s</button>`,
			locale.Tr("repo.commits.avatar_stack_people", len(participants)))
		b.WriteHTML(`<div class="tippy-target"><div class="avatar-stack-popup">`)
		for _, participant := range participants {
			b.WriteHTML(ut.participantPopupRow(data, participant))
		}
		b.WriteHTML(`</div></div>`)
	}

	b.WriteHTML(`</span>`)
	return b.HTMLString()
}

// participantNameLink prefers (in order): commits-by-author search, `GetShortDisplayNameLinkHTML` (keeps alt-name tooltip), `mailto:`, bare name.
func (ut *RenderUtils) participantNameLink(data *user_model.AvatarStackData, participant *user_model.CommitParticipant) template.HTML {
	if href := renderAvatarStackViewEmailLink(data, participant.GitIdentity.Email); href != "" {
		return htmlutil.HTMLFormat(`<a class="muted" href="%s">%s</a>`, href, participantName(participant))
	}
	if participant.GiteaUser != nil {
		return participant.GiteaUser.GetShortDisplayNameLinkHTML()
	}
	if participant.GitIdentity.Email != "" {
		return htmlutil.HTMLFormat(`<a class="muted" href="mailto:%s">%s</a>`, participant.GitIdentity.Email, participant.GitIdentity.Name)
	}
	return template.HTML(template.HTMLEscapeString(participant.GitIdentity.Name))
}

func (ut *RenderUtils) participantPopupRow(data *user_model.AvatarStackData, participant *user_model.CommitParticipant) template.HTML {
	avatar := ut.participantAvatar(participant)
	name := participantName(participant)
	if href := ut.participantHref(data, participant); href != "" {
		return htmlutil.HTMLFormat(`<a class="silenced flex-text-block" href="%s">%s<span>%s</span></a>`, href, avatar, name)
	}
	return htmlutil.HTMLFormat(`<span class="flex-text-block">%s<span>%s</span></span>`, avatar, name)
}
