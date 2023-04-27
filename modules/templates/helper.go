// Copyright 2018 The Gitea Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"math"
	"mime"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/avatars"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/git"
	giturl "code.gitea.io/gitea/modules/git/url"
	gitea_html "code.gitea.io/gitea/modules/html"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/templates/eval"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

// Used from static.go && dynamic.go
var mailSubjectSplit = regexp.MustCompile(`(?m)^-{3,}[\s]*$`)

// NewFuncMap returns functions for injecting to templates
func NewFuncMap() []template.FuncMap {
	return []template.FuncMap{map[string]interface{}{
		// -----------------------------------------------------------------
		// html/template related functions
		"dict":        dict, // it's lowercase because this name has been widely used. Our other functions should have uppercase names.
		"Eval":        Eval,
		"Safe":        Safe,
		"Escape":      html.EscapeString,
		"QueryEscape": url.QueryEscape,
		"JSEscape":    template.JSEscapeString,
		"Str2html":    Str2html, // TODO: rename it to SanitizeHTML
		"URLJoin":     util.URLJoin,

		"PathEscape":         url.PathEscape,
		"PathEscapeSegments": util.PathEscapeSegments,

		// utils
		"StringUtils": NewStringUtils,
		"SliceUtils":  NewSliceUtils,

		// -----------------------------------------------------------------
		// string / json
		// TODO: move string helper functions to StringUtils
		"Join":           strings.Join,
		"DotEscape":      DotEscape,
		"EllipsisString": base.EllipsisString,
		"DumpVar":        dumpVar,

		"Json": func(in interface{}) string {
			out, err := json.Marshal(in)
			if err != nil {
				return ""
			}
			return string(out)
		},
		"JsonPrettyPrint": func(in string) string {
			var out bytes.Buffer
			err := json.Indent(&out, []byte(in), "", "  ")
			if err != nil {
				return ""
			}
			return out.String()
		},

		// -----------------------------------------------------------------
		// svg / avatar / icon
		"svg":            svg.RenderHTML,
		"avatar":         Avatar,
		"avatarHTML":     AvatarHTML,
		"avatarByAction": AvatarByAction,
		"avatarByEmail":  AvatarByEmail,
		"repoAvatar":     RepoAvatar,
		"EntryIcon":      base.EntryIcon,
		"MigrationIcon":  MigrationIcon,
		"ActionIcon":     ActionIcon,

		"SortArrow": func(normSort, revSort, urlSort string, isDefault bool) template.HTML {
			// if needed
			if len(normSort) == 0 || len(urlSort) == 0 {
				return ""
			}

			if len(urlSort) == 0 && isDefault {
				// if sort is sorted as default add arrow tho this table header
				if isDefault {
					return svg.RenderHTML("octicon-triangle-down", 16)
				}
			} else {
				// if sort arg is in url test if it correlates with column header sort arguments
				// the direction of the arrow should indicate the "current sort order", up means ASC(normal), down means DESC(rev)
				if urlSort == normSort {
					// the table is sorted with this header normal
					return svg.RenderHTML("octicon-triangle-up", 16)
				} else if urlSort == revSort {
					// the table is sorted with this header reverse
					return svg.RenderHTML("octicon-triangle-down", 16)
				}
			}
			// the table is NOT sorted with this header
			return ""
		},

		// -----------------------------------------------------------------
		// time / number / format
		"FileSize":      base.FileSize,
		"CountFmt":      base.FormatNumberSI,
		"TimeSince":     timeutil.TimeSince,
		"TimeSinceUnix": timeutil.TimeSinceUnix,
		"DateTime":      timeutil.DateTime,
		"Sec2Time":      util.SecToTime,
		"LoadTimes": func(startTime time.Time) string {
			return fmt.Sprint(time.Since(startTime).Nanoseconds()/1e6) + "ms"
		},

		// -----------------------------------------------------------------
		// setting
		"AppName": func() string {
			return setting.AppName
		},
		"AppSubUrl": func() string {
			return setting.AppSubURL
		},
		"AssetUrlPrefix": func() string {
			return setting.StaticURLPrefix + "/assets"
		},
		"AppUrl": func() string {
			// The usage of AppUrl should be avoided as much as possible,
			// because the AppURL(ROOT_URL) may not match user's visiting site and the ROOT_URL in app.ini may be incorrect.
			// And it's difficult for Gitea to guess absolute URL correctly with zero configuration,
			// because Gitea doesn't know whether the scheme is HTTP or HTTPS unless the reverse proxy could tell Gitea.
			return setting.AppURL
		},
		"AppVer": func() string {
			return setting.AppVer
		},
		"AppDomain": func() string { // documented in mail-templates.md
			return setting.Domain
		},
		"AssetVersion": func() string {
			return setting.AssetVersion
		},
		"DisableGravatar": func(ctx context.Context) bool {
			return system_model.GetSettingWithCacheBool(ctx, system_model.KeyPictureDisableGravatar)
		},
		"DefaultShowFullName": func() bool {
			return setting.UI.DefaultShowFullName
		},
		"ShowFooterTemplateLoadTime": func() bool {
			return setting.Other.ShowFooterTemplateLoadTime
		},
		"AllowedReactions": func() []string {
			return setting.UI.Reactions
		},
		"CustomEmojis": func() map[string]string {
			return setting.UI.CustomEmojisMap
		},
		"ThemeColorMetaTag": func() string {
			return setting.UI.ThemeColorMetaTag
		},
		"MetaAuthor": func() string {
			return setting.UI.Meta.Author
		},
		"MetaDescription": func() string {
			return setting.UI.Meta.Description
		},
		"MetaKeywords": func() string {
			return setting.UI.Meta.Keywords
		},
		"UseServiceWorker": func() bool {
			return setting.UI.UseServiceWorker
		},
		"EnableTimetracking": func() bool {
			return setting.Service.EnableTimetracking
		},
		"DisableGitHooks": func() bool {
			return setting.DisableGitHooks
		},
		"DisableWebhooks": func() bool {
			return setting.DisableWebhooks
		},
		"DisableImportLocal": func() bool {
			return !setting.ImportLocalPaths
		},
		"DefaultTheme": func() string {
			return setting.UI.DefaultTheme
		},
		"NotificationSettings": func() map[string]interface{} {
			return map[string]interface{}{
				"MinTimeout":            int(setting.UI.Notification.MinTimeout / time.Millisecond),
				"TimeoutStep":           int(setting.UI.Notification.TimeoutStep / time.Millisecond),
				"MaxTimeout":            int(setting.UI.Notification.MaxTimeout / time.Millisecond),
				"EventSourceUpdateTime": int(setting.UI.Notification.EventSourceUpdateTime / time.Millisecond),
			}
		},
		"MermaidMaxSourceCharacters": func() int {
			return setting.MermaidMaxSourceCharacters
		},

		// -----------------------------------------------------------------
		// render
		"RenderCommitMessage":            RenderCommitMessage,
		"RenderCommitMessageLinkSubject": RenderCommitMessageLinkSubject,

		"RenderCommitBody": RenderCommitBody,
		"RenderCodeBlock":  RenderCodeBlock,
		"RenderIssueTitle": RenderIssueTitle,
		"RenderEmoji":      RenderEmoji,
		"RenderEmojiPlain": emoji.ReplaceAliases,
		"ReactionToEmoji":  ReactionToEmoji,
		"RenderNote":       RenderNote,

		"RenderMarkdownToHtml": func(ctx context.Context, input string) template.HTML {
			output, err := markdown.RenderString(&markup.RenderContext{
				Ctx:       ctx,
				URLPrefix: setting.AppSubURL,
			}, input)
			if err != nil {
				log.Error("RenderString: %v", err)
			}
			return template.HTML(output)
		},
		"RenderLabel": func(ctx context.Context, label *issues_model.Label) template.HTML {
			return template.HTML(RenderLabel(ctx, label))
		},
		"RenderLabels": func(ctx context.Context, labels []*issues_model.Label, repoLink string) template.HTML {
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
		},

		// -----------------------------------------------------------------
		// misc
		"DiffLineTypeToStr":        DiffLineTypeToStr,
		"ShortSha":                 base.ShortSha,
		"ActionContent2Commits":    ActionContent2Commits,
		"IsMultilineCommitMessage": IsMultilineCommitMessage,
		"CommentMustAsDiff":        gitdiff.CommentMustAsDiff,
		"MirrorRemoteAddress":      mirrorRemoteAddress,

		"ParseDeadline": func(deadline string) []string {
			return strings.Split(deadline, "|")
		},
		"FilenameIsImage": func(filename string) bool {
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			return strings.HasPrefix(mimeType, "image/")
		},
		"TabSizeClass": func(ec interface{}, filename string) string {
			var (
				value *editorconfig.Editorconfig
				ok    bool
			)
			if ec != nil {
				if value, ok = ec.(*editorconfig.Editorconfig); !ok || value == nil {
					return "tab-size-8"
				}
				def, err := value.GetDefinitionForFilename(filename)
				if err != nil {
					log.Error("tab size class: getting definition for filename: %v", err)
					return "tab-size-8"
				}
				if def.TabWidth > 0 {
					return fmt.Sprintf("tab-size-%d", def.TabWidth)
				}
			}
			return "tab-size-8"
		},
		"SubJumpablePath": func(str string) []string {
			var path []string
			index := strings.LastIndex(str, "/")
			if index != -1 && index != len(str) {
				path = append(path, str[0:index+1], str[index+1:])
			} else {
				path = append(path, str)
			}
			return path
		},
		"CompareLink": func(baseRepo, repo *repo_model.Repository, branchName string) string {
			var curBranch string
			if repo.ID != baseRepo.ID {
				curBranch += fmt.Sprintf("%s/%s:", url.PathEscape(repo.OwnerName), url.PathEscape(repo.Name))
			}
			curBranch += util.PathEscapeSegments(branchName)

			return fmt.Sprintf("%s/compare/%s...%s",
				baseRepo.Link(),
				util.PathEscapeSegments(baseRepo.DefaultBranch),
				curBranch,
			)
		},
	}}
}

// AvatarHTML creates the HTML for an avatar
func AvatarHTML(src string, size int, class, name string) template.HTML {
	sizeStr := fmt.Sprintf(`%d`, size)

	if name == "" {
		name = "avatar"
	}

	return template.HTML(`<img class="` + class + `" src="` + src + `" title="` + html.EscapeString(name) + `" width="` + sizeStr + `" height="` + sizeStr + `"/>`)
}

// Avatar renders user avatars. args: user, size (int), class (string)
func Avatar(ctx context.Context, item interface{}, others ...interface{}) template.HTML {
	size, class := gitea_html.ParseSizeAndClass(avatars.DefaultAvatarPixelSize, avatars.DefaultAvatarClass, others...)

	switch t := item.(type) {
	case *user_model.User:
		src := t.AvatarLinkWithSize(ctx, size*setting.Avatar.RenderedSizeFactor)
		if src != "" {
			return AvatarHTML(src, size, class, t.DisplayName())
		}
	case *repo_model.Collaborator:
		src := t.AvatarLinkWithSize(ctx, size*setting.Avatar.RenderedSizeFactor)
		if src != "" {
			return AvatarHTML(src, size, class, t.DisplayName())
		}
	case *organization.Organization:
		src := t.AsUser().AvatarLinkWithSize(ctx, size*setting.Avatar.RenderedSizeFactor)
		if src != "" {
			return AvatarHTML(src, size, class, t.AsUser().DisplayName())
		}
	}

	return template.HTML("")
}

// AvatarByAction renders user avatars from action. args: action, size (int), class (string)
func AvatarByAction(ctx context.Context, action *activities_model.Action, others ...interface{}) template.HTML {
	action.LoadActUser(ctx)
	return Avatar(ctx, action.ActUser, others...)
}

// RepoAvatar renders repo avatars. args: repo, size(int), class (string)
func RepoAvatar(repo *repo_model.Repository, others ...interface{}) template.HTML {
	size, class := gitea_html.ParseSizeAndClass(avatars.DefaultAvatarPixelSize, avatars.DefaultAvatarClass, others...)

	src := repo.RelAvatarLink()
	if src != "" {
		return AvatarHTML(src, size, class, repo.FullName())
	}
	return template.HTML("")
}

// AvatarByEmail renders avatars by email address. args: email, name, size (int), class (string)
func AvatarByEmail(ctx context.Context, email, name string, others ...interface{}) template.HTML {
	size, class := gitea_html.ParseSizeAndClass(avatars.DefaultAvatarPixelSize, avatars.DefaultAvatarClass, others...)
	src := avatars.GenerateEmailAvatarFastLink(ctx, email, size*setting.Avatar.RenderedSizeFactor)

	if src != "" {
		return AvatarHTML(src, size, class, name)
	}

	return template.HTML("")
}

// Safe render raw as HTML
func Safe(raw string) template.HTML {
	return template.HTML(raw)
}

// Str2html render Markdown text to HTML
func Str2html(raw string) template.HTML {
	return template.HTML(markup.Sanitize(raw))
}

// DotEscape wraps a dots in names with ZWJ [U+200D] in order to prevent autolinkers from detecting these as urls
func DotEscape(raw string) string {
	return strings.ReplaceAll(raw, ".", "\u200d.\u200d")
}

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
func RenderLabel(ctx context.Context, label *issues_model.Label) string {
	labelScope := label.ExclusiveScope()

	textColor := "#111"
	if label.UseLightTextColor() {
		textColor = "#eee"
	}

	description := emoji.ReplaceAliases(template.HTMLEscapeString(label.Description))

	if labelScope == "" {
		// Regular label
		return fmt.Sprintf("<div class='ui label' style='color: %s !important; background-color: %s !important' title='%s'>%s</div>",
			textColor, label.Color, description, RenderEmoji(ctx, label.Name))
	}

	// Scoped label
	scopeText := RenderEmoji(ctx, labelScope)
	itemText := RenderEmoji(ctx, label.Name[len(labelScope)+1:])

	itemColor := label.Color
	scopeColor := label.Color
	if r, g, b, err := label.ColorRGB(); err == nil {
		// Make scope and item background colors slightly darker and lighter respectively.
		// More contrast needed with higher luminance, empirically tweaked.
		luminance := (0.299*r + 0.587*g + 0.114*b) / 255
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

		itemColor = "#" + hex.EncodeToString(itemBytes)
		scopeColor = "#" + hex.EncodeToString(scopeBytes)
	}

	return fmt.Sprintf("<span class='ui label scope-parent' title='%s'>"+
		"<div class='ui label scope-left' style='color: %s !important; background-color: %s !important'>%s</div>"+
		"<div class='ui label scope-right' style='color: %s !important; background-color: %s !important''>%s</div>"+
		"</span>",
		description,
		textColor, scopeColor, scopeText,
		textColor, itemColor, itemText)
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

// IsMultilineCommitMessage checks to see if a commit message contains multiple lines.
func IsMultilineCommitMessage(msg string) bool {
	return strings.Count(strings.TrimSpace(msg), "\n") >= 1
}

// Actioner describes an action
type Actioner interface {
	GetOpType() activities_model.ActionType
	GetActUserName() string
	GetRepoUserName() string
	GetRepoName() string
	GetRepoPath() string
	GetRepoLink() string
	GetBranch() string
	GetContent() string
	GetCreate() time.Time
	GetIssueInfos() []string
}

// ActionIcon accepts an action operation type and returns an icon class name.
func ActionIcon(opType activities_model.ActionType) string {
	switch opType {
	case activities_model.ActionCreateRepo, activities_model.ActionTransferRepo, activities_model.ActionRenameRepo:
		return "repo"
	case activities_model.ActionCommitRepo, activities_model.ActionPushTag, activities_model.ActionDeleteTag, activities_model.ActionDeleteBranch:
		return "git-commit"
	case activities_model.ActionCreateIssue:
		return "issue-opened"
	case activities_model.ActionCreatePullRequest:
		return "git-pull-request"
	case activities_model.ActionCommentIssue, activities_model.ActionCommentPull:
		return "comment-discussion"
	case activities_model.ActionMergePullRequest, activities_model.ActionAutoMergePullRequest:
		return "git-merge"
	case activities_model.ActionCloseIssue, activities_model.ActionClosePullRequest:
		return "issue-closed"
	case activities_model.ActionReopenIssue, activities_model.ActionReopenPullRequest:
		return "issue-reopened"
	case activities_model.ActionMirrorSyncPush, activities_model.ActionMirrorSyncCreate, activities_model.ActionMirrorSyncDelete:
		return "mirror"
	case activities_model.ActionApprovePullRequest:
		return "check"
	case activities_model.ActionRejectPullRequest:
		return "diff"
	case activities_model.ActionPublishRelease:
		return "tag"
	case activities_model.ActionPullReviewDismissed:
		return "x"
	default:
		return "question"
	}
}

// ActionContent2Commits converts action content to push commits
func ActionContent2Commits(act Actioner) *repository.PushCommits {
	push := repository.NewPushCommits()

	if act == nil || act.GetContent() == "" {
		return push
	}

	if err := json.Unmarshal([]byte(act.GetContent()), push); err != nil {
		log.Error("json.Unmarshal:\n%s\nERROR: %v", act.GetContent(), err)
	}

	if push.Len == 0 {
		push.Len = len(push.Commits)
	}

	return push
}

// DiffLineTypeToStr returns diff line type name
func DiffLineTypeToStr(diffType int) string {
	switch diffType {
	case 2:
		return "add"
	case 3:
		return "del"
	case 4:
		return "tag"
	}
	return "same"
}

// MigrationIcon returns a SVG name matching the service an issue/comment was migrated from
func MigrationIcon(hostname string) string {
	switch hostname {
	case "github.com":
		return "octicon-mark-github"
	default:
		return "gitea-git"
	}
}

type remoteAddress struct {
	Address  string
	Username string
	Password string
}

func mirrorRemoteAddress(ctx context.Context, m *repo_model.Repository, remoteName string, ignoreOriginalURL bool) remoteAddress {
	a := remoteAddress{}

	remoteURL := m.OriginalURL
	if ignoreOriginalURL || remoteURL == "" {
		var err error
		remoteURL, err = git.GetRemoteAddress(ctx, m.RepoPath(), remoteName)
		if err != nil {
			log.Error("GetRemoteURL %v", err)
			return a
		}
	}

	u, err := giturl.Parse(remoteURL)
	if err != nil {
		log.Error("giturl.Parse %v", err)
		return a
	}

	if u.Scheme != "ssh" && u.Scheme != "file" {
		if u.User != nil {
			a.Username = u.User.Username()
			a.Password, _ = u.User.Password()
		}
		u.User = nil
	}
	a.Address = u.String()

	return a
}

// Eval the expression and return the result, see the comment of eval.Expr for details.
// To use this helper function in templates, pass each token as a separate parameter.
//
//	{{ $int64 := Eval $var "+" 1 }}
//	{{ $float64 := Eval $var "+" 1.0 }}
//
// Golang's template supports comparable int types, so the int64 result can be used in later statements like {{if lt $int64 10}}
func Eval(tokens ...any) (any, error) {
	n, err := eval.Expr(tokens...)
	return n.Value, err
}
