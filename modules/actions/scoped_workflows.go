// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
)

// ListScopedWorkflows lists scoped workflow files (under SCOPED_WORKFLOW_DIRS) at the given commit.
func ListScopedWorkflows(commit *git.Commit) (string, git.Entries, error) {
	return listWorkflowsInDirs(commit, setting.Actions.ScopedWorkflowDirs)
}

// ParsedScopedWorkflow is one scoped workflow's source-side parse result
type ParsedScopedWorkflow struct {
	EntryName   string
	DisplayName string             // the workflow `name:` or base file name
	Content     []byte             // raw content of the workflow file
	Events      []*jobparser.Event // decoded `on:` events
}

// ParseScopedWorkflows lists and parses the scoped workflow files at sourceCommit (under SCOPED_WORKFLOW_DIRS).
func ParseScopedWorkflows(sourceCommit *git.Commit) ([]*ParsedScopedWorkflow, error) {
	_, entries, err := ListScopedWorkflows(sourceCommit)
	if err != nil {
		return nil, err
	}

	parsed := make([]*ParsedScopedWorkflow, 0, len(entries))
	for _, entry := range entries {
		content, err := GetContentFromEntry(entry)
		if err != nil {
			return nil, err
		}

		// one workflow may have multiple events
		events, err := GetEventsFromContent(content)
		if err != nil {
			log.Warn("ignore invalid scoped workflow %q: %v", entry.Name(), err)
			continue
		}
		parsed = append(parsed, &ParsedScopedWorkflow{
			EntryName:   entry.Name(),
			DisplayName: WorkflowDisplayName(entry.Name(), content),
			Content:     content,
			Events:      events,
		})
	}
	return parsed, nil
}

// MatchScopedWorkflows evaluates already-parsed scoped workflows against one consuming event, returning those whose `on:` matches.
func MatchScopedWorkflows(
	parsed []*ParsedScopedWorkflow,
	consumerGitRepo *git.Repository,
	consumerCommit *git.Commit,
	triggedEvent webhook_module.HookEventType,
	payload api.Payloader,
) []*DetectedWorkflow {
	workflows := make([]*DetectedWorkflow, 0, len(parsed))
	for _, p := range parsed {
		for _, evt := range p.Events {
			if evt.IsSchedule() {
				// schedule is a non-target for scoped workflows
				continue
			}
			if detectWorkflowMatch(consumerGitRepo, consumerCommit, triggedEvent, payload, evt) == detectMatched {
				workflows = append(workflows, &DetectedWorkflow{
					EntryName:    p.EntryName,
					TriggerEvent: evt,
					Content:      p.Content,
				})
			}
		}
	}
	return workflows
}
