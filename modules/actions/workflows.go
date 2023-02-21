// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/gobwas/glob"
	"github.com/nektos/act/pkg/jobparser"
	"github.com/nektos/act/pkg/model"
)

func ListWorkflows(commit *git.Commit) (git.Entries, error) {
	tree, err := commit.SubTree(".gitea/workflows")
	if _, ok := err.(git.ErrNotExist); ok {
		tree, err = commit.SubTree(".github/workflows")
	}
	if _, ok := err.(git.ErrNotExist); ok {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	entries, err := tree.ListEntriesRecursiveFast()
	if err != nil {
		return nil, err
	}

	ret := make(git.Entries, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml") {
			ret = append(ret, entry)
		}
	}
	return ret, nil
}

func converGithubEvent2GiteaEvent(evt *jobparser.Event) string {
	switch evt.Name {
	case "create":
		return string(webhook_module.HookEventCreate)
	case "delete":
		return string(webhook_module.HookEventDelete)
	case "fork":
		return string(webhook_module.HookEventFork)
	case "issue_comment":
		return string(webhook_module.HookEventIssueComment)
	case "issues":
		for _, tp := range evt.Acts["types"] {
			switch tp {
			case "assigned":
				return string(webhook_module.HookEventIssueAssign)
			case "milestoned":
				return string(webhook_module.HookEventIssueMilestone)
			case "labeled":
				return string(webhook_module.HookEventIssueLabel)
			}
		}
		return string(webhook_module.HookEventIssues)
	case "pull_request", "pull_request_target":
		for _, tp := range evt.Acts["types"] {
			switch tp {
			case "assigned":
				return string(webhook_module.HookEventPullRequestAssign)
			case "milestoned":
				return string(webhook_module.HookEventPullRequestMilestone)
			case "labeled":
				return string(webhook_module.HookEventPullRequestLabel)
			case "synchronize":
				return string(webhook_module.HookEventPullRequestSync)
			}
		}
		return string(webhook_module.HookEventPullRequest)
	case "pull_request_comment":
		return string(webhook_module.HookEventPullRequestComment)
	case "pull_request_review_comment":
		return string(webhook_module.HookEventPullRequestReviewComment)
	case "push":
		return string(webhook_module.HookEventPush)
	case "registry_package":
		return string(webhook_module.HookEventPackage)
	case "release":
		return string(webhook_module.HookEventRelease)
	case "pull_request_review", "milestone", "label", "project", "project_card", "project_column":
		fallthrough
	default:
		return evt.Name
	}
}

func DetectWorkflows(commit *git.Commit, triggedEvent webhook_module.HookEventType, payload api.Payloader) (map[string][]byte, error) {
	entries, err := ListWorkflows(commit)
	if err != nil {
		return nil, err
	}

	workflows := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		f, err := entry.Blob().DataAsync()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		workflow, err := model.ReadWorkflow(bytes.NewReader(content))
		if err != nil {
			log.Warn("ignore invalid workflow %q: %v", entry.Name(), err)
			continue
		}
		events, err := jobparser.ParseRawOn(&workflow.RawOn)
		if err != nil {
			log.Warn("ignore invalid workflow %q: %v", entry.Name(), err)
			continue
		}
		for _, evt := range events {
			log.Trace("detect workflow %q for event %q matching %q", entry.Name(), evt.Name, triggedEvent)
			if detectMatched(commit, triggedEvent, payload, evt) {
				workflows[entry.Name()] = content
			}
		}
	}

	return workflows, nil
}

func matchPushEvent(commit *git.Commit, pushPayload *api.PushPayload, evt *jobparser.Event) bool {
	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts {
		switch cond {
		case "branches", "tags":
			refShortName := git.RefName(pushPayload.Ref).ShortName()
			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(refShortName) {
					matchTimes++
					break
				}
			}
		case "paths":
			filesChanged, err := commit.GetFilesChangedSinceCommit(pushPayload.Before)
			if err != nil {
				log.Error("GetFilesChangedSinceCommit [commit_sha1: %s]: %v", commit.ID.String(), err)
			} else {
				for _, val := range vals {
					matched := false
					for _, file := range filesChanged {
						if glob.MustCompile(val, '/').Match(file) {
							matched = true
							break
						}
					}
					if matched {
						matchTimes++
						break
					}
				}
			}
		default:
			log.Warn("unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts)
}

func matchIssuesEvent(commit *git.Commit, issuePayload *api.IssuePayload, evt *jobparser.Event) bool {
	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts {
		switch cond {
		case "types":
			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(string(issuePayload.Action)) {
					matchTimes++
					break
				}
			}
		default:
			log.Warn("unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts)
}

func matchPullRequestEvent(commit *git.Commit, prPayload *api.PullRequestPayload, evt *jobparser.Event) bool {
	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts {
		switch cond {
		case "types":
			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(string(prPayload.Action)) {
					matchTimes++
					break
				}
			}
		case "branches":
			refShortName := git.RefName(prPayload.PullRequest.Base.Ref).ShortName()
			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(refShortName) {
					matchTimes++
					break
				}
			}
		case "paths":
			filesChanged, err := commit.GetFilesChangedSinceCommit(prPayload.PullRequest.Base.Ref)
			if err != nil {
				log.Error("GetFilesChangedSinceCommit [commit_sha1: %s]: %v", commit.ID.String(), err)
			} else {
				for _, val := range vals {
					matched := false
					for _, file := range filesChanged {
						if glob.MustCompile(val, '/').Match(file) {
							matched = true
							break
						}
					}
					if matched {
						matchTimes++
						break
					}
				}
			}
		default:
			log.Warn("unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts)
}

func matchIssueCommentEvent(commit *git.Commit, issueCommentPayload *api.IssueCommentPayload, evt *jobparser.Event) bool {
	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts {
		switch cond {
		case "types":
			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(string(issueCommentPayload.Action)) {
					matchTimes++
					break
				}
			}
		default:
			log.Warn("unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts)
}

func detectMatched(commit *git.Commit, triggedEvent webhook_module.HookEventType, payload api.Payloader, evt *jobparser.Event) bool {
	if converGithubEvent2GiteaEvent(evt) != triggedEvent.Event() {
		return false
	}

	// with no special filter parameters
	if len(evt.Acts) == 0 {
		return true
	}

	switch triggedEvent {
	case webhook_module.HookEventCreate,
		webhook_module.HookEventDelete,
		webhook_module.HookEventFork,
		webhook_module.HookEventIssueAssign,
		webhook_module.HookEventIssueLabel,
		webhook_module.HookEventIssueMilestone,
		webhook_module.HookEventPullRequestAssign,
		webhook_module.HookEventPullRequestLabel,
		webhook_module.HookEventPullRequestMilestone,
		webhook_module.HookEventPullRequestComment,
		webhook_module.HookEventPullRequestReviewApproved,
		webhook_module.HookEventPullRequestReviewRejected,
		webhook_module.HookEventPullRequestReviewComment,
		webhook_module.HookEventWiki,
		webhook_module.HookEventRepository,
		webhook_module.HookEventRelease,
		webhook_module.HookEventPackage:
		// no special filter parameters for these events, just return true if name matched
		return true

	case webhook_module.HookEventPush:
		return matchPushEvent(commit, payload.(*api.PushPayload), evt)

	case webhook_module.HookEventIssues:
		return matchIssuesEvent(commit, payload.(*api.IssuePayload), evt)

	case webhook_module.HookEventPullRequest, webhook_module.HookEventPullRequestSync:
		return matchPullRequestEvent(commit, payload.(*api.PullRequestPayload), evt)

	case webhook_module.HookEventIssueComment:
		return matchIssueCommentEvent(commit, payload.(*api.IssueCommentPayload), evt)

	default:
		log.Warn("unsupported event %q", triggedEvent.Event())
		return false
	}
}
