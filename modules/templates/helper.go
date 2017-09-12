// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"bytes"
	"container/list"
	"encoding/json"
	"fmt"
	"html/template"
	"mime"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
	"gopkg.in/editorconfig/editorconfig-core-go.v1"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"
)

// NewFuncMap returns functions for injecting to templates
func NewFuncMap() []template.FuncMap {
	return []template.FuncMap{map[string]interface{}{
		"GoVer": func() string {
			return strings.Title(runtime.Version())
		},
		"UseHTTPS": func() bool {
			return strings.HasPrefix(setting.AppURL, "https")
		},
		"AppName": func() string {
			return setting.AppName
		},
		"AppSubUrl": func() string {
			return setting.AppSubURL
		},
		"AppUrl": func() string {
			return setting.AppURL
		},
		"AppVer": func() string {
			return setting.AppVer
		},
		"AppBuiltWith": func() string {
			return setting.AppBuiltWith
		},
		"AppDomain": func() string {
			return setting.Domain
		},
		"DisableGravatar": func() bool {
			return setting.DisableGravatar
		},
		"ShowFooterTemplateLoadTime": func() bool {
			return setting.ShowFooterTemplateLoadTime
		},
		"LoadTimes": func(startTime time.Time) string {
			return fmt.Sprint(time.Since(startTime).Nanoseconds()/1e6) + "ms"
		},
		"AvatarLink":   base.AvatarLink,
		"Safe":         Safe,
		"SafeJS":       SafeJS,
		"Sanitize":     bluemonday.UGCPolicy().Sanitize,
		"Str2html":     Str2html,
		"TimeSince":    base.TimeSince,
		"RawTimeSince": base.RawTimeSince,
		"FileSize":     base.FileSize,
		"Subtract":     base.Subtract,
		"Add": func(a, b int) int {
			return a + b
		},
		"ActionIcon": ActionIcon,
		"DateFmtLong": func(t time.Time) string {
			return t.Format(time.RFC1123Z)
		},
		"DateFmtShort": func(t time.Time) string {
			return t.Format("Jan 02, 2006")
		},
		"SizeFmt": func(s int64) string {
			return base.FileSize(s)
		},
		"List": List,
		"SubStr": func(str string, start, length int) string {
			if len(str) == 0 {
				return ""
			}
			end := start + length
			if length == -1 {
				end = len(str)
			}
			if len(str) < end {
				return str
			}
			return str[start:end]
		},
		"EllipsisString":    base.EllipsisString,
		"DiffTypeToStr":     DiffTypeToStr,
		"DiffLineTypeToStr": DiffLineTypeToStr,
		"Sha1":              Sha1,
		"ShortSha":          base.ShortSha,
		"MD5":               base.EncodeMD5,
		"ActionContent2Commits": ActionContent2Commits,
		"EscapePound": func(str string) string {
			return strings.NewReplacer("%", "%25", "#", "%23", " ", "%20", "?", "%3F").Replace(str)
		},
		"RenderCommitMessage": RenderCommitMessage,
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
		"FilenameIsImage": func(filename string) bool {
			mimeType := mime.TypeByExtension(filepath.Ext(filename))
			return strings.HasPrefix(mimeType, "image/")
		},
		"TabSizeClass": func(ec *editorconfig.Editorconfig, filename string) string {
			if ec != nil {
				def := ec.GetDefinitionForFilename(filename)
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
				path = append(path, str[0:index+1])
				path = append(path, str[index+1:])
			} else {
				path = append(path, str)
			}
			return path
		},
		"JsonPrettyPrint": func(in string) string {
			var out bytes.Buffer
			err := json.Indent(&out, []byte(in), "", "  ")
			if err != nil {
				return ""
			}
			return out.String()
		},
		"DisableGitHooks": func() bool {
			return setting.DisableGitHooks
		},
	}}
}

// Safe render raw as HTML
func Safe(raw string) template.HTML {
	return template.HTML(raw)
}

// SafeJS renders raw as JS
func SafeJS(raw string) template.JS {
	return template.JS(raw)
}

// Str2html render Markdown text to HTML
func Str2html(raw string) template.HTML {
	return template.HTML(markdown.Sanitize(raw))
}

// List traversings the list
func List(l *list.List) chan interface{} {
	e := l.Front()
	c := make(chan interface{})
	go func() {
		for e != nil {
			c <- e.Value
			e = e.Next()
		}
		close(c)
	}()
	return c
}

// Sha1 returns sha1 sum of string
func Sha1(str string) string {
	return base.EncodeSha1(str)
}

// ToUTF8WithErr converts content to UTF8 encoding
func ToUTF8WithErr(content []byte) (string, error) {
	charsetLabel, err := base.DetectEncoding(content)
	if err != nil {
		return "", err
	} else if charsetLabel == "UTF-8" {
		return string(content), nil
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		return string(content), fmt.Errorf("Unknown encoding: %s", charsetLabel)
	}

	// If there is an error, we concatenate the nicely decoded part and the
	// original left over. This way we won't loose data.
	result, n, err := transform.String(encoding.NewDecoder(), string(content))
	if err != nil {
		result = result + string(content[n:])
	}

	return result, err
}

// ToUTF8 converts content to UTF8 encoding and ignore error
func ToUTF8(content string) string {
	res, _ := ToUTF8WithErr([]byte(content))
	return res
}

// ReplaceLeft replaces all prefixes 'old' in 's' with 'new'.
func ReplaceLeft(s, old, new string) string {
	oldLen, newLen, i, n := len(old), len(new), 0, 0
	for ; i < len(s) && strings.HasPrefix(s[i:], old); n++ {
		i += oldLen
	}

	// simple optimization
	if n == 0 {
		return s
	}

	// allocating space for the new string
	curLen := n*newLen + len(s[i:])
	replacement := make([]byte, curLen, curLen)

	j := 0
	for ; j < n*newLen; j += newLen {
		copy(replacement[j:j+newLen], new)
	}

	copy(replacement[j:], s[i:])
	return string(replacement)
}

// RenderCommitMessage renders commit message with XSS-safe and special links.
func RenderCommitMessage(full bool, msg, urlPrefix string, metas map[string]string) template.HTML {
	cleanMsg := template.HTMLEscapeString(msg)
	fullMessage := string(markdown.RenderIssueIndexPattern([]byte(cleanMsg), urlPrefix, metas))
	msgLines := strings.Split(strings.TrimSpace(fullMessage), "\n")
	numLines := len(msgLines)
	if numLines == 0 {
		return template.HTML("")
	} else if !full {
		return template.HTML(msgLines[0])
	} else if numLines == 1 || (numLines >= 2 && len(msgLines[1]) == 0) {
		// First line is a header, standalone or followed by empty line
		header := fmt.Sprintf("<h3>%s</h3>", msgLines[0])
		if numLines >= 2 {
			fullMessage = header + fmt.Sprintf("\n<pre>%s</pre>", strings.Join(msgLines[2:], "\n"))
		} else {
			fullMessage = header
		}
	} else {
		// Non-standard git message, there is no header line
		fullMessage = fmt.Sprintf("<h4>%s</h4>", strings.Join(msgLines, "<br>"))
	}
	return template.HTML(fullMessage)
}

// Actioner describes an action
type Actioner interface {
	GetOpType() int
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

// ActionIcon accepts a int that represents action operation type
// and returns a icon class name.
func ActionIcon(opType int) string {
	switch opType {
	case 1, 8: // Create and transfer repository
		return "repo"
	case 5, 9: // Commit repository
		return "git-commit"
	case 6: // Create issue
		return "issue-opened"
	case 7: // New pull request
		return "git-pull-request"
	case 10: // Comment issue
		return "comment-discussion"
	case 11: // Merge pull request
		return "git-merge"
	case 12, 14: // Close issue or pull request
		return "issue-closed"
	case 13, 15: // Reopen issue or pull request
		return "issue-reopened"
	default:
		return "invalid type"
	}
}

// ActionContent2Commits converts action content to push commits
func ActionContent2Commits(act Actioner) *models.PushCommits {
	push := models.NewPushCommits()
	if err := json.Unmarshal([]byte(act.GetContent()), push); err != nil {
		log.Error(4, "json.Unmarshal:\n%s\nERROR: %v", act.GetContent(), err)
	}
	return push
}

// DiffTypeToStr returns diff type name
func DiffTypeToStr(diffType int) string {
	diffTypes := map[int]string{
		1: "add", 2: "modify", 3: "del", 4: "rename",
	}
	return diffTypes[diffType]
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
