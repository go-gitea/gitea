// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/actions/workflowpattern"
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
			if evt.Name != triggedEvent.Event() {
				continue
			}
			if detectMatched(commit, triggedEvent, payload, evt) {
				workflows[entry.Name()] = content
			}
		}
	}

	return workflows, nil
}

func detectMatched(commit *git.Commit, triggedEvent webhook_module.HookEventType, payload api.Payloader, evt *jobparser.Event) bool {
	if len(evt.Acts) == 0 {
		return true
	}

	switch triggedEvent {
	case webhook_module.HookEventCreate:
		fallthrough
	case webhook_module.HookEventDelete:
		fallthrough
	case webhook_module.HookEventFork:
		log.Warn("unsupported event %q", triggedEvent.Event())
		return false
	case webhook_module.HookEventPush:
		pushPayload := payload.(*api.PushPayload)
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
				log.Warn("unsupported condition %q", cond)
			}
		}
		// if both branch and tag filter are defined in the workflow only one needs to match
		if hasBranchFilter && hasTagFilter {
			matchTimes++
		}
		return matchTimes == len(evt.Acts)

	case webhook_module.HookEventIssues:
		fallthrough
	case webhook_module.HookEventIssueAssign:
		fallthrough
	case webhook_module.HookEventIssueLabel:
		fallthrough
	case webhook_module.HookEventIssueMilestone:
		fallthrough
	case webhook_module.HookEventIssueComment:
		fallthrough
	case webhook_module.HookEventPullRequest:
		prPayload := payload.(*api.PullRequestPayload)
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
				log.Warn("unsupported condition %q", cond)
			}
		}
		return matchTimes == len(evt.Acts)
	case webhook_module.HookEventPullRequestAssign:
		fallthrough
	case webhook_module.HookEventPullRequestLabel:
		fallthrough
	case webhook_module.HookEventPullRequestMilestone:
		fallthrough
	case webhook_module.HookEventPullRequestComment:
		fallthrough
	case webhook_module.HookEventPullRequestReviewApproved:
		fallthrough
	case webhook_module.HookEventPullRequestReviewRejected:
		fallthrough
	case webhook_module.HookEventPullRequestReviewComment:
		fallthrough
	case webhook_module.HookEventPullRequestSync:
		fallthrough
	case webhook_module.HookEventWiki:
		fallthrough
	case webhook_module.HookEventRepository:
		fallthrough
	case webhook_module.HookEventRelease:
		fallthrough
	case webhook_module.HookEventPackage:
		fallthrough
	default:
		log.Warn("unsupported event %q", triggedEvent.Event())
	}
	return false
}
