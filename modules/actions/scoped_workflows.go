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

// MatchScopedWorkflows evaluates already-parsed scoped workflows against one consuming event.
// It returns the workflows whose `on:` matches, and those that matched the event but were excluded by a branch/paths filter (filtered).
func MatchScopedWorkflows(
	parsed []*ParsedScopedWorkflow,
	consumerGitRepo *git.Repository,
	consumerCommit *git.Commit,
	triggedEvent webhook_module.HookEventType,
	payload api.Payloader,
) (matched, filtered []*DetectedWorkflow) {
	for _, p := range parsed {
		for _, evt := range p.Events {
			if evt.IsSchedule() {
				// schedule is a non-target for scoped workflows
				continue
			}
			dwf := &DetectedWorkflow{
				EntryName:    p.EntryName,
				TriggerEvent: evt,
				Content:      p.Content,
			}
			switch detectWorkflowMatch(consumerGitRepo, consumerCommit, triggedEvent, payload, evt) {
			case detectMatched:
				matched = append(matched, dwf)
			case detectFilteredOut:
				filtered = append(filtered, dwf)
			case detectNotApplicable:
			}
		}
	}
	return matched, filtered
}
