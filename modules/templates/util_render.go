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

// commitAuthorSearchURL builds the repo's commits-by-author search URL. Returns
// empty when no repo/ref context is present or when the value would not survive
// the search parser (it splits on whitespace via strings.FieldsSeq).
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

// authorHref picks the most-relevant link target for an author. The search
// query uses the commit's signature email (no spaces, matches `git log
// --author` substring on email) over username/display name. Falls back to the
// user profile (matched) or mailto (unmatched).
func (ut *RenderUtils) authorHref(u *user_model.User, sig *git.Signature) template.URL {
	var searchValue, fallback string
	if sig != nil && sig.Email != "" {
		searchValue = sig.Email
	} else if u != nil {
		searchValue = u.Email
	}
	switch {
	case u != nil:
		fallback = u.HomeLink()
	case sig != nil && sig.Email != "":
		fallback = "mailto:" + sig.Email
	default:
		return ""
	}
	if href := ut.commitAuthorSearchURL(searchValue); href != "" {
		return href
	}
	return template.URL(fallback)
}

// authorAvatar returns the 20px avatar HTML for a matched user or git signature.
func (ut *RenderUtils) authorAvatar(u *user_model.User, sig *git.Signature) template.HTML {
	au := NewAvatarUtils(ut.ctx)
	if u != nil {
		return au.Avatar(u, 20)
	}
	return au.AvatarByEmail(sig.Email, sig.Name, 20)
}

// authorDisplayName returns the display name for a matched user or git signature.
func authorDisplayName(u *user_model.User, sig *git.Signature) string {
	if u != nil {
		return u.GetDisplayName()
	}
	return sig.Name
}

// AvatarStack renders an avatar stack. Returns empty when no author is provided.
// Each stack child carries inline `--n` so the CSS can drive z-index and the hover spread.
func (ut *RenderUtils) AvatarStack(authorUser *user_model.User, authorSig *git.Signature, coAuthors []*user_model.CoAuthorUser, additionalClasses string) template.HTML {
	if authorUser == nil && authorSig == nil {
		return ""
	}
	if len(coAuthors) == 0 {
		au := NewAvatarUtils(ut.ctx)
		if authorUser != nil {
			return au.Avatar(authorUser, 20, additionalClasses)
		}
		return au.AvatarByEmail(authorSig.Email, authorSig.Name, 20, additionalClasses)
	}

	const maxCo = 9
	visibleCo := coAuthors
	overflow := 0
	if len(coAuthors) > maxCo {
		visibleCo = coAuthors[:maxCo]
		overflow = len(coAuthors) - maxCo
	}

	stackClass := "avatar-stack"
	if additionalClasses != "" {
		stackClass += " " + additionalClasses
	}

	var b strings.Builder
	b.WriteString(string(htmlutil.HTMLFormat(`<span class="%s">`, stackClass)))

	appendChild := func(idx int, u *user_model.User, sig *git.Signature) {
		avatar := ut.authorAvatar(u, sig)
		if href := ut.authorHref(u, sig); href != "" {
			b.WriteString(string(htmlutil.HTMLFormat(`<a href="%s" style="--n:%d">%s</a>`, href, idx, avatar)))
		} else {
			b.WriteString(string(htmlutil.HTMLFormat(`<span style="--n:%d">%s</span>`, idx, avatar)))
		}
	}

	appendChild(0, authorUser, authorSig)
	for i, co := range visibleCo {
		appendChild(i+1, co.GiteaUser, co.TrailerSignature)
	}

	if overflow > 0 {
		b.WriteString(string(htmlutil.HTMLFormat(
			`<span class="avatar-stack-overflow-chip tw-text-xs" style="--n:%d" aria-label="+%d">+%d</span>`,
			len(visibleCo)+1, overflow, overflow)))
	}

	b.WriteString(`</span>`)
	return template.HTML(b.String())
}

// CoAuthorAvatars renders the avatar stack plus descriptive label.
// 1 author: name; 2 total: `<a> and <b>`; 3+ total: `<N> people` opens a tippy popup.
func (ut *RenderUtils) CoAuthorAvatars(authorUser *user_model.User, authorSig *git.Signature, coAuthors []*user_model.CoAuthorUser) template.HTML {
	locale := ut.ctx.Value(translation.ContextKey).(translation.Locale)
	stackClass := ""
	if len(coAuthors) == 0 {
		stackClass = "tw-mr-1"
	}

	var b strings.Builder
	b.WriteString(`<span class="author-wrapper">`)
	b.WriteString(string(ut.AvatarStack(authorUser, authorSig, coAuthors, stackClass)))

	switch len(coAuthors) {
	case 0:
		b.WriteString(string(ut.authorNameLinkHTML(authorUser, authorSig)))
	case 1:
		b.WriteString(string(ut.authorNameLinkHTML(authorUser, authorSig)))
		b.WriteString(string(htmlutil.HTMLFormat(`<span class="tw-mx-1">%s</span>`, locale.Tr("repo.commits.coauthor_and"))))
		b.WriteString(string(ut.authorNameLinkHTML(coAuthors[0].GiteaUser, coAuthors[0].TrailerSignature)))
	default:
		b.WriteString(string(htmlutil.HTMLFormat(
			`<button type="button" class="authors-popup-trigger" data-global-init="initAuthorsPopup">%s</button>`,
			locale.Tr("repo.commits.coauthor_people", len(coAuthors)+1))))
		b.WriteString(`<div class="tippy-target"><div class="authors-popup">`)
		b.WriteString(string(ut.participantRowHTML(authorUser, authorSig)))
		for _, co := range coAuthors {
			b.WriteString(string(ut.participantRowHTML(co.GiteaUser, co.TrailerSignature)))
		}
		b.WriteString(`</div></div>`)
	}

	b.WriteString(`</span>`)
	return template.HTML(b.String())
}

// authorNameLinkHTML renders a muted text link for an author. In a repo+ref
// context it points at the commits-by-author search (keyed by signature email);
// otherwise it falls back to `GetShortDisplayNameLinkHTML` for matched users
// (preserving the alternate-name tooltip) or a `mailto:` link for unmatched.
func (ut *RenderUtils) authorNameLinkHTML(u *user_model.User, sig *git.Signature) template.HTML {
	var email string
	if sig != nil {
		email = sig.Email
	} else if u != nil {
		email = u.Email
	}
	if href := ut.commitAuthorSearchURL(email); href != "" {
		if u != nil {
			return htmlutil.HTMLFormat(`<a class="muted" href="%s">%s</a>`, href, u.GetDisplayName())
		}
		return htmlutil.HTMLFormat(`<a class="muted" href="%s">%s</a>`, href, sig.Name)
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

// participantRowHTML renders one row of the authors popup: avatar + name.
func (ut *RenderUtils) participantRowHTML(u *user_model.User, sig *git.Signature) template.HTML {
	return htmlutil.HTMLFormat(`<a class="silenced flex-text-block" href="%s">%s<span>%s</span></a>`,
		ut.authorHref(u, sig), ut.authorAvatar(u, sig), authorDisplayName(u, sig))
}
