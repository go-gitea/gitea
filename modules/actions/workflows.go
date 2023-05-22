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
	"github.com/nektos/act/pkg/workflowpattern"
	"gopkg.in/yaml.v3"
)

func init() {
	model.OnDecodeNodeError = func(node yaml.Node, out interface{}, err error) {
		// Log the error instead of panic or fatal.
		// It will be a big job to refactor act/pkg/model to return decode error,
		// so we just log the error and return empty value, and improve it later.
		log.Error("Failed to decode node %v into %T: %v", node, out, err)
	}
}

func IsWorkflow(path string) bool {
	if (!strings.HasSuffix(path, ".yaml")) && (!strings.HasSuffix(path, ".yml")) {
		return false
	}

	return strings.HasPrefix(path, ".gitea/workflows") || strings.HasPrefix(path, ".github/workflows")
}

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

func GetContentFromEntry(entry *git.TreeEntry) ([]byte, error) {
	f, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, err
	}
	content, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	return content, nil
}

func GetEventsFromContent(content []byte) ([]*jobparser.Event, error) {
	workflow, err := model.ReadWorkflow(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	events, err := jobparser.ParseRawOn(&workflow.RawOn)
	if err != nil {
		return nil, err
	}

	return events, nil
}

func DetectWorkflows(commit *git.Commit, triggedEvent webhook_module.HookEventType, payload api.Payloader) (map[string][]byte, error) {
	entries, err := ListWorkflows(commit)
	if err != nil {
		return nil, err
	}

	workflows := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		content, err := GetContentFromEntry(entry)
		if err != nil {
			return nil, err
		}
		events, err := GetEventsFromContent(content)
		if err != nil {
			log.Warn("ignore invalid workflow %q: %v", entry.Name(), err)
			continue
		}
		for _, evt := range events {
			log.Trace("detect workflow %q for event %#v matching %q", entry.Name(), evt, triggedEvent)
			if detectMatched(commit, triggedEvent, payload, evt) {
				workflows[entry.Name()] = content
			}
		}
	}

	return workflows, nil
}

func detectMatched(commit *git.Commit, triggedEvent webhook_module.HookEventType, payload api.Payloader, evt *jobparser.Event) bool {
	if !canGithubEventMatch(evt.Name, triggedEvent) {
		return false
	}

	switch triggedEvent {
	case // events with no activity types
		webhook_module.HookEventCreate,
		webhook_module.HookEventDelete,
		webhook_module.HookEventFork,
		webhook_module.HookEventWiki:
		if len(evt.Acts()) != 0 {
			log.Warn("Ignore unsupported %s event arguments %v", triggedEvent, evt.Acts())
		}
		// no special filter parameters for these events, just return true if name matched
		return true

	case // push
		webhook_module.HookEventPush:
		return matchPushEvent(commit, payload.(*api.PushPayload), evt)

	case // issues
		webhook_module.HookEventIssues,
		webhook_module.HookEventIssueAssign,
		webhook_module.HookEventIssueLabel,
		webhook_module.HookEventIssueMilestone:
		return matchIssuesEvent(commit, payload.(*api.IssuePayload), evt)

	case // issue_comment
		webhook_module.HookEventIssueComment,
		// `pull_request_comment` is same as `issue_comment`
		// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request_comment-use-issue_comment
		webhook_module.HookEventPullRequestComment:
		return matchIssueCommentEvent(commit, payload.(*api.IssueCommentPayload), evt)

	case // pull_request
		webhook_module.HookEventPullRequest,
		webhook_module.HookEventPullRequestSync,
		webhook_module.HookEventPullRequestAssign,
		webhook_module.HookEventPullRequestLabel:
		return matchPullRequestEvent(commit, payload.(*api.PullRequestPayload), evt)

	case // pull_request_review
		webhook_module.HookEventPullRequestReviewApproved,
		webhook_module.HookEventPullRequestReviewRejected:
		return matchPullRequestReviewEvent(commit, payload.(*api.PullRequestPayload), evt)

	case // pull_request_review_comment
		webhook_module.HookEventPullRequestReviewComment:
		return matchPullRequestReviewCommentEvent(commit, payload.(*api.PullRequestPayload), evt)

	case // release
		webhook_module.HookEventRelease:
		return matchReleaseEvent(commit, payload.(*api.ReleasePayload), evt)

	case // registry_package
		webhook_module.HookEventPackage:
		return matchPackageEvent(commit, payload.(*api.PackagePayload), evt)

	default:
		log.Warn("unsupported event %q", triggedEvent)
		return false
	}
}

func matchPushEvent(commit *git.Commit, pushPayload *api.PushPayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts()) == 0 {
		return true
	}

	matchTimes := 0
	hasBranchFilter := false
	hasTagFilter := false
	refName := git.RefName(pushPayload.Ref)
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts() {
		switch cond {
		case "branches":
			hasBranchFilter = true
			if !refName.IsBranch() {
				break
			}
			patterns, err := workflowpattern.CompilePatterns(vals...)
			if err != nil {
				break
			}
			if !workflowpattern.Skip(patterns, []string{refName.ShortName()}, &workflowpattern.EmptyTraceWriter{}) {
				matchTimes++
			}
		case "branches-ignore":
			hasBranchFilter = true
			if !refName.IsBranch() {
				break
			}
			patterns, err := workflowpattern.CompilePatterns(vals...)
			if err != nil {
				break
			}
			if !workflowpattern.Filter(patterns, []string{refName.ShortName()}, &workflowpattern.EmptyTraceWriter{}) {
				matchTimes++
			}
		case "tags":
			hasTagFilter = true
			if !refName.IsTag() {
				break
			}
			patterns, err := workflowpattern.CompilePatterns(vals...)
			if err != nil {
				break
			}
			if !workflowpattern.Skip(patterns, []string{refName.ShortName()}, &workflowpattern.EmptyTraceWriter{}) {
				matchTimes++
			}
		case "tags-ignore":
			hasTagFilter = true
			if !refName.IsTag() {
				break
			}
			patterns, err := workflowpattern.CompilePatterns(vals...)
			if err != nil {
				break
			}
			if !workflowpattern.Filter(patterns, []string{refName.ShortName()}, &workflowpattern.EmptyTraceWriter{}) {
				matchTimes++
			}
		case "paths":
			filesChanged, err := commit.GetFilesChangedSinceCommit(pushPayload.Before)
			if err != nil {
				log.Error("GetFilesChangedSinceCommit [commit_sha1: %s]: %v", commit.ID.String(), err)
			} else {
				patterns, err := workflowpattern.CompilePatterns(vals...)
				if err != nil {
					break
				}
				if !workflowpattern.Skip(patterns, filesChanged, &workflowpattern.EmptyTraceWriter{}) {
					matchTimes++
				}
			}
		case "paths-ignore":
			filesChanged, err := commit.GetFilesChangedSinceCommit(pushPayload.Before)
			if err != nil {
				log.Error("GetFilesChangedSinceCommit [commit_sha1: %s]: %v", commit.ID.String(), err)
			} else {
				patterns, err := workflowpattern.CompilePatterns(vals...)
				if err != nil {
					break
				}
				if !workflowpattern.Filter(patterns, filesChanged, &workflowpattern.EmptyTraceWriter{}) {
					matchTimes++
				}
			}
		default:
			log.Warn("push event unsupported condition %q", cond)
		}
	}
	// if both branch and tag filter are defined in the workflow only one needs to match
	if hasBranchFilter && hasTagFilter {
		matchTimes++
	}
	return matchTimes == len(evt.Acts())
}

func matchIssuesEvent(commit *git.Commit, issuePayload *api.IssuePayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts()) == 0 {
		return true
	}

	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts() {
		switch cond {
		case "types":
			// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#issues
			// Actions with the same name:
			// opened, edited, closed, reopened, assigned, unassigned, milestoned, demilestoned
			// Actions need to be converted:
			// label_updated -> labeled
			// label_cleared -> unlabeled
			// Unsupported activity types:
			// deleted, transferred, pinned, unpinned, locked, unlocked

			action := issuePayload.Action
			switch action {
			case api.HookIssueLabelUpdated:
				action = "labeled"
			case api.HookIssueLabelCleared:
				action = "unlabeled"
			}
			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(string(action)) {
					matchTimes++
					break
				}
			}
		default:
			log.Warn("issue event unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts())
}

func matchPullRequestEvent(commit *git.Commit, prPayload *api.PullRequestPayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts()) == 0 {
		// defaultly, only pull request `opened`, `reopened` and `synchronized` will trigger workflow
		// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request
		return prPayload.Action == api.HookIssueSynchronized || prPayload.Action == api.HookIssueOpened || prPayload.Action == api.HookIssueReOpened
	}

	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts() {
		switch cond {
		case "types":
			// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request
			// Actions with the same name:
			// opened, edited, closed, reopened, assigned, unassigned
			// Actions need to be converted:
			// synchronized -> synchronize
			// label_updated -> labeled
			// label_cleared -> unlabeled
			// Unsupported activity types:
			// converted_to_draft, ready_for_review, locked, unlocked, review_requested, review_request_removed, auto_merge_enabled, auto_merge_disabled

			action := prPayload.Action
			switch action {
			case api.HookIssueSynchronized:
				action = "synchronize"
			case api.HookIssueLabelUpdated:
				action = "labeled"
			case api.HookIssueLabelCleared:
				action = "unlabeled"
			}
			log.Trace("matching pull_request %s with %v", action, vals)
			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(string(action)) {
					matchTimes++
					break
				}
			}
		case "branches":
			refName := git.RefName(prPayload.PullRequest.Base.Ref)
			patterns, err := workflowpattern.CompilePatterns(vals...)
			if err != nil {
				break
			}
			if !workflowpattern.Skip(patterns, []string{refName.ShortName()}, &workflowpattern.EmptyTraceWriter{}) {
				matchTimes++
			}
		case "branches-ignore":
			refName := git.RefName(prPayload.PullRequest.Base.Ref)
			patterns, err := workflowpattern.CompilePatterns(vals...)
			if err != nil {
				break
			}
			if !workflowpattern.Filter(patterns, []string{refName.ShortName()}, &workflowpattern.EmptyTraceWriter{}) {
				matchTimes++
			}
		case "paths":
			filesChanged, err := commit.GetFilesChangedSinceCommit(prPayload.PullRequest.Base.Ref)
			if err != nil {
				log.Error("GetFilesChangedSinceCommit [commit_sha1: %s]: %v", commit.ID.String(), err)
			} else {
				patterns, err := workflowpattern.CompilePatterns(vals...)
				if err != nil {
					break
				}
				if !workflowpattern.Skip(patterns, filesChanged, &workflowpattern.EmptyTraceWriter{}) {
					matchTimes++
				}
			}
		case "paths-ignore":
			filesChanged, err := commit.GetFilesChangedSinceCommit(prPayload.PullRequest.Base.Ref)
			if err != nil {
				log.Error("GetFilesChangedSinceCommit [commit_sha1: %s]: %v", commit.ID.String(), err)
			} else {
				patterns, err := workflowpattern.CompilePatterns(vals...)
				if err != nil {
					break
				}
				if !workflowpattern.Filter(patterns, filesChanged, &workflowpattern.EmptyTraceWriter{}) {
					matchTimes++
				}
			}
		default:
			log.Warn("pull request event unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts())
}

func matchIssueCommentEvent(commit *git.Commit, issueCommentPayload *api.IssueCommentPayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts()) == 0 {
		return true
	}

	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts() {
		switch cond {
		case "types":
			// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#issue_comment
			// Actions with the same name:
			// created, edited, deleted
			// Actions need to be converted:
			// NONE
			// Unsupported activity types:
			// NONE

			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(string(issueCommentPayload.Action)) {
					matchTimes++
					break
				}
			}
		default:
			log.Warn("issue comment event unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts())
}

func matchPullRequestReviewEvent(commit *git.Commit, prPayload *api.PullRequestPayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts()) == 0 {
		return true
	}

	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts() {
		switch cond {
		case "types":
			// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request_review
			// Activity types with the same name:
			// NONE
			// Activity types need to be converted:
			// reviewed -> submitted
			// reviewed -> edited
			// Unsupported activity types:
			// dismissed

			actions := make([]string, 0)
			if prPayload.Action == api.HookIssueReviewed {
				// the `reviewed` HookIssueAction can match the two activity types: `submitted` and `edited`
				// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request_review
				actions = append(actions, "submitted", "edited")
			}

			matched := false
			for _, val := range vals {
				for _, action := range actions {
					if glob.MustCompile(val, '/').Match(action) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if matched {
				matchTimes++
			}
		default:
			log.Warn("pull request review event unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts())
}

func matchPullRequestReviewCommentEvent(commit *git.Commit, prPayload *api.PullRequestPayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts()) == 0 {
		return true
	}

	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts() {
		switch cond {
		case "types":
			// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request_review_comment
			// Activity types with the same name:
			// NONE
			// Activity types need to be converted:
			// reviewed -> created
			// reviewed -> edited
			// Unsupported activity types:
			// deleted

			actions := make([]string, 0)
			if prPayload.Action == api.HookIssueReviewed {
				// the `reviewed` HookIssueAction can match the two activity types: `created` and `edited`
				// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request_review_comment
				actions = append(actions, "created", "edited")
			}

			matched := false
			for _, val := range vals {
				for _, action := range actions {
					if glob.MustCompile(val, '/').Match(action) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if matched {
				matchTimes++
			}
		default:
			log.Warn("pull request review comment event unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts())
}

func matchReleaseEvent(commit *git.Commit, payload *api.ReleasePayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts()) == 0 {
		return true
	}

	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts() {
		switch cond {
		case "types":
			// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#release
			// Activity types with the same name:
			// published
			// Activity types need to be converted:
			// updated -> edited
			// Unsupported activity types:
			// unpublished, created, deleted, prereleased, released

			action := payload.Action
			switch action {
			case api.HookReleaseUpdated:
				action = "edited"
			}
			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(string(action)) {
					matchTimes++
					break
				}
			}
		default:
			log.Warn("release event unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts())
}

func matchPackageEvent(commit *git.Commit, payload *api.PackagePayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts()) == 0 {
		return true
	}

	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts() {
		switch cond {
		case "types":
			// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#registry_package
			// Activity types with the same name:
			// NONE
			// Activity types need to be converted:
			// created -> published
			// Unsupported activity types:
			// updated

			action := payload.Action
			switch action {
			case api.HookPackageCreated:
				action = "published"
			}
			for _, val := range vals {
				if glob.MustCompile(val, '/').Match(string(action)) {
					matchTimes++
					break
				}
			}
		default:
			log.Warn("package event unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts())
}
