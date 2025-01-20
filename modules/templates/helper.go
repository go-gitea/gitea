// Copyright 2018 The Gitea Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"fmt"
	"html"
	"html/template"
	"net/url"
	"reflect"
	"strings"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/templates/eval"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/gitdiff"
	"code.gitea.io/gitea/services/webtheme"
)

// NewFuncMap returns functions for injecting to templates
func NewFuncMap() template.FuncMap {
	return map[string]any{
		"ctx": func() any { return nil }, // template context function

		"DumpVar": dumpVar,
		"NIL":     func() any { return nil },

		// -----------------------------------------------------------------
		// html/template related functions
		"dict":         dict, // it's lowercase because this name has been widely used. Our other functions should have uppercase names.
		"Iif":          iif,
		"Eval":         evalTokens,
		"SafeHTML":     safeHTML,
		"HTMLFormat":   htmlutil.HTMLFormat,
		"HTMLEscape":   htmlEscape,
		"QueryEscape":  queryEscape,
		"QueryBuild":   QueryBuild,
		"JSEscape":     jsEscapeSafe,
		"SanitizeHTML": SanitizeHTML,
		"URLJoin":      util.URLJoin,
		"DotEscape":    dotEscape,

		"PathEscape":         url.PathEscape,
		"PathEscapeSegments": util.PathEscapeSegments,

		// utils
		"StringUtils": NewStringUtils,
		"SliceUtils":  NewSliceUtils,
		"JsonUtils":   NewJsonUtils,
		"DateUtils":   NewDateUtils,

		// -----------------------------------------------------------------
		// svg / avatar / icon / color
		"svg":           svg.RenderHTML,
		"EntryIcon":     base.EntryIcon,
		"MigrationIcon": migrationIcon,
		"ActionIcon":    actionIcon,
		"SortArrow":     sortArrow,
		"ContrastColor": util.ContrastColor,

		// -----------------------------------------------------------------
		// time / number / format
		"FileSize": base.FileSize,
		"CountFmt": base.FormatNumberSI,
		"Sec2Time": util.SecToHours,

		"TimeEstimateString": timeEstimateString,

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
		"DefaultShowFullName": func() bool {
			return setting.UI.DefaultShowFullName
		},
		"ShowFooterTemplateLoadTime": func() bool {
			return setting.Other.ShowFooterTemplateLoadTime
		},
		"ShowFooterPoweredBy": func() bool {
			return setting.Other.ShowFooterPoweredBy
		},
		"AllowedReactions": func() []string {
			return setting.UI.Reactions
		},
		"CustomEmojis": func() map[string]string {
			return setting.UI.CustomEmojisMap
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
		"UserThemeName": userThemeName,
		"NotificationSettings": func() map[string]any {
			return map[string]any{
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
		"RenderCodeBlock": renderCodeBlock,
		"ReactionToEmoji": reactionToEmoji,

		// -----------------------------------------------------------------
		// misc
		"ShortSha":                 base.ShortSha,
		"ActionContent2Commits":    ActionContent2Commits,
		"IsMultilineCommitMessage": isMultilineCommitMessage,
		"CommentMustAsDiff":        gitdiff.CommentMustAsDiff,
		"MirrorRemoteAddress":      mirrorRemoteAddress,

		"FilenameIsImage": filenameIsImage,
		"TabSizeClass":    tabSizeClass,

		// for backward compatibility only, do not use them anymore
		"TimeSince":     timeSinceLegacy,
		"TimeSinceUnix": timeSinceLegacy,
		"DateTime":      dateTimeLegacy,

		"RenderEmoji":      renderEmojiLegacy,
		"RenderLabel":      renderLabelLegacy,
		"RenderLabels":     renderLabelsLegacy,
		"RenderIssueTitle": renderIssueTitleLegacy,

		"RenderMarkdownToHtml": renderMarkdownToHtmlLegacy,

		"RenderCommitMessage":            renderCommitMessageLegacy,
		"RenderCommitMessageLinkSubject": renderCommitMessageLinkSubjectLegacy,
		"RenderCommitBody":               renderCommitBodyLegacy,
	}
}

// safeHTML render raw as HTML
func safeHTML(s any) template.HTML {
	switch v := s.(type) {
	case string:
		return template.HTML(v)
	case template.HTML:
		return v
	}
	panic(fmt.Sprintf("unexpected type %T", s))
}

// SanitizeHTML sanitizes the input by pre-defined markdown rules
func SanitizeHTML(s string) template.HTML {
	return template.HTML(markup.Sanitize(s))
}

func htmlEscape(s any) template.HTML {
	switch v := s.(type) {
	case string:
		return template.HTML(html.EscapeString(v))
	case template.HTML:
		return v
	}
	panic(fmt.Sprintf("unexpected type %T", s))
}

func jsEscapeSafe(s string) template.HTML {
	return template.HTML(template.JSEscapeString(s))
}

func queryEscape(s string) template.URL {
	return template.URL(url.QueryEscape(s))
}

// dotEscape wraps a dots in names with ZWJ [U+200D] in order to prevent auto-linkers from detecting these as urls
func dotEscape(raw string) string {
	return strings.ReplaceAll(raw, ".", "\u200d.\u200d")
}

// iif is an "inline-if", similar util.Iif[T] but templates need the non-generic version,
// and it could be simply used as "{{iif expr trueVal}}" (omit the falseVal).
func iif(condition any, vals ...any) any {
	if isTemplateTruthy(condition) {
		return vals[0]
	} else if len(vals) > 1 {
		return vals[1]
	}
	return nil
}

func isTemplateTruthy(v any) bool {
	if v == nil {
		return false
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() != 0
	case reflect.Complex64, reflect.Complex128:
		return rv.Complex() != 0
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return rv.Len() > 0
	case reflect.Struct:
		return true
	default:
		return !rv.IsNil()
	}
}

// evalTokens evaluates the expression by tokens and returns the result, see the comment of eval.Expr for details.
// To use this helper function in templates, pass each token as a separate parameter.
//
//	{{ $int64 := Eval $var "+" 1 }}
//	{{ $float64 := Eval $var "+" 1.0 }}
//
// Golang's template supports comparable int types, so the int64 result can be used in later statements like {{if lt $int64 10}}
func evalTokens(tokens ...any) (any, error) {
	n, err := eval.Expr(tokens...)
	return n.Value, err
}

func userThemeName(user *user_model.User) string {
	if user == nil || user.Theme == "" {
		return setting.UI.DefaultTheme
	}
	if webtheme.IsThemeAvailable(user.Theme) {
		return user.Theme
	}
	return setting.UI.DefaultTheme
}

func timeEstimateString(timeSec any) string {
	v, _ := util.ToInt64(timeSec)
	if v == 0 {
		return ""
	}
	return util.TimeEstimateString(v)
}

// QueryBuild builds a query string from a list of key-value pairs.
// It omits the nil and empty strings, but it doesn't omit other zero values,
// because the zero value of number types may have a meaning.
func QueryBuild(a ...any) template.URL {
	var s string
	if len(a)%2 == 1 {
		if v, ok := a[0].(string); ok {
			if v == "" || (v[0] != '?' && v[0] != '&') {
				panic("QueryBuild: invalid argument")
			}
			s = v
		} else if v, ok := a[0].(template.URL); ok {
			s = string(v)
		} else {
			panic("QueryBuild: invalid argument")
		}
	}
	for i := len(a) % 2; i < len(a); i += 2 {
		k, ok := a[i].(string)
		if !ok {
			panic("QueryBuild: invalid argument")
		}
		var v string
		if va, ok := a[i+1].(string); ok {
			v = va
		} else if a[i+1] != nil {
			v = fmt.Sprint(a[i+1])
		}
		// pos1 to pos2 is the "k=v&" part, "&" is optional
		pos1 := strings.Index(s, "&"+k+"=")
		if pos1 != -1 {
			pos1++
		} else {
			pos1 = strings.Index(s, "?"+k+"=")
			if pos1 != -1 {
				pos1++
			} else if strings.HasPrefix(s, k+"=") {
				pos1 = 0
			}
		}
		pos2 := len(s)
		if pos1 == -1 {
			pos1 = len(s)
		} else {
			pos2 = pos1 + 1
			for pos2 < len(s) && s[pos2-1] != '&' {
				pos2++
			}
		}
		if v != "" {
			sep := ""
			hasPrefixSep := pos1 == 0 || (pos1 <= len(s) && (s[pos1-1] == '?' || s[pos1-1] == '&'))
			if !hasPrefixSep {
				sep = "&"
			}
			s = s[:pos1] + sep + k + "=" + url.QueryEscape(v) + "&" + s[pos2:]
		} else {
			s = s[:pos1] + s[pos2:]
		}
	}
	if s != "" && s != "&" && s[len(s)-1] == '&' {
		s = s[:len(s)-1]
	}
	return template.URL(s)
}

func panicIfDevOrTesting() {
	if !setting.IsProd || setting.IsInTesting {
		panic("legacy template functions are for backward compatibility only, do not use them in new code")
	}
}
