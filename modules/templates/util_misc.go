// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"html/template"
	"mime"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	giturl "code.gitea.io/gitea/modules/git/url"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/svg"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

func sortArrow(normSort, revSort, urlSort string, isDefault bool) template.HTML {
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
		switch urlSort {
		case normSort:
			// the table is sorted with this header normal
			return svg.RenderHTML("octicon-triangle-up", 16)
		case revSort:
			// the table is sorted with this header reverse
			return svg.RenderHTML("octicon-triangle-down", 16)
		}
	}
	// the table is NOT sorted with this header
	return ""
}

// isMultilineCommitMessage checks to see if a commit message contains multiple lines.
func isMultilineCommitMessage(msg string) bool {
	return strings.Count(strings.TrimSpace(msg), "\n") >= 1
}

// Actioner describes an action
type Actioner interface {
	GetOpType() activities_model.ActionType
	GetActUserName(ctx context.Context) string
	GetRepoUserName(ctx context.Context) string
	GetRepoName(ctx context.Context) string
	GetRepoPath(ctx context.Context) string
	GetRepoLink(ctx context.Context) string
	GetBranch() string
	GetContent() string
	GetCreate() time.Time
	GetIssueInfos() []string
}

// actionIcon accepts an action operation type and returns an icon class name.
func actionIcon(opType activities_model.ActionType) string {
	switch opType {
	case activities_model.ActionCreateRepo, activities_model.ActionTransferRepo, activities_model.ActionRenameRepo:
		return "repo"
	case activities_model.ActionCommitRepo:
		return "git-commit"
	case activities_model.ActionDeleteBranch:
		return "git-branch"
	case activities_model.ActionMergePullRequest, activities_model.ActionAutoMergePullRequest:
		return "git-merge"
	case activities_model.ActionCreatePullRequest:
		return "git-pull-request"
	case activities_model.ActionClosePullRequest:
		return "git-pull-request-closed"
	case activities_model.ActionCreateIssue:
		return "issue-opened"
	case activities_model.ActionCloseIssue:
		return "issue-closed"
	case activities_model.ActionReopenIssue, activities_model.ActionReopenPullRequest:
		return "issue-reopened"
	case activities_model.ActionCommentIssue, activities_model.ActionCommentPull:
		return "comment-discussion"
	case activities_model.ActionMirrorSyncPush, activities_model.ActionMirrorSyncCreate, activities_model.ActionMirrorSyncDelete:
		return "mirror"
	case activities_model.ActionApprovePullRequest:
		return "check"
	case activities_model.ActionRejectPullRequest:
		return "file-diff"
	case activities_model.ActionPublishRelease, activities_model.ActionPushTag, activities_model.ActionDeleteTag:
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

// migrationIcon returns a SVG name matching the service an issue/comment was migrated from
func migrationIcon(hostname string) string {
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

func mirrorRemoteAddress(ctx context.Context, m *repo_model.Repository, remoteName string) remoteAddress {
	ret := remoteAddress{}
	remoteURL, err := git.GetRemoteAddress(ctx, m.RepoPath(), remoteName)
	if err != nil {
		log.Error("GetRemoteURL %v", err)
		return ret
	}

	u, err := giturl.ParseGitURL(remoteURL)
	if err != nil {
		log.Error("giturl.Parse %v", err)
		return ret
	}

	if u.Scheme != "ssh" && u.Scheme != "file" {
		if u.User != nil {
			ret.Username = u.User.Username()
			ret.Password, _ = u.User.Password()
		}
	}

	// The URL stored in the git repo could contain authentication,
	// erase it, or it will be shown in the UI.
	u.User = nil
	ret.Address = u.String()
	// Why not use m.OriginalURL to set ret.Address?
	// It should be OK to use it, since m.OriginalURL should be the same as the authentication-erased URL from the Git repository.
	// However, the old code has already stored authentication in m.OriginalURL when updating mirror settings.
	// That means we need to use "giturl.Parse" for m.OriginalURL again to ensure authentication is erased.
	// Instead of doing this, why not directly use the authentication-erased URL from the Git repository?
	// It should be the same as long as there are no bugs.

	return ret
}

func filenameIsImage(filename string) bool {
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	return strings.HasPrefix(mimeType, "image/")
}

func tabSizeClass(ec *editorconfig.Editorconfig, filename string) string {
	if ec != nil {
		def, err := ec.GetDefinitionForFilename(filename)
		if err == nil && def.TabWidth >= 1 && def.TabWidth <= 16 {
			return "tab-size-" + strconv.Itoa(def.TabWidth)
		}
	}
	return "tab-size-4"
}
