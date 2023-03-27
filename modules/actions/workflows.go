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
		if len(evt.Acts) != 0 {
			log.Warn("Ignore unsupported %s event arguments %q", triggedEvent, evt.Acts)
		}
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
		log.Warn("unsupported event %q", triggedEvent)
		return false
	}
}

func matchPushEvent(commit *git.Commit, pushPayload *api.PushPayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts) == 0 {
		return true
	}

	matchTimes := 0
	hasBranchFilter := false
	hasTagFilter := false
	refName := git.RefName(pushPayload.Ref)
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts {
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
	return matchTimes == len(evt.Acts)
}

func matchIssuesEvent(commit *git.Commit, issuePayload *api.IssuePayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts) == 0 {
		return true
	}

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
			log.Warn("issue event unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts)
}

func matchPullRequestEvent(commit *git.Commit, prPayload *api.PullRequestPayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts) == 0 {
		// defaultly, only pull request opened and synchronized will trigger workflow
		return prPayload.Action == api.HookIssueSynchronized || prPayload.Action == api.HookIssueOpened
	}

	matchTimes := 0
	// all acts conditions should be satisfied
	for cond, vals := range evt.Acts {
		switch cond {
		case "types":
			action := prPayload.Action
			if prPayload.Action == api.HookIssueSynchronized {
				action = "synchronize"
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
	return matchTimes == len(evt.Acts)
}

func matchIssueCommentEvent(commit *git.Commit, issueCommentPayload *api.IssueCommentPayload, evt *jobparser.Event) bool {
	// with no special filter parameters
	if len(evt.Acts) == 0 {
		return true
	}

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
			log.Warn("issue comment unsupported condition %q", cond)
		}
	}
	return matchTimes == len(evt.Acts)
}
