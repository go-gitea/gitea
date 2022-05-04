// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"strings"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"

	"github.com/nektos/act/pkg/model"
)

func DetectWorkflows(commit *git.Commit, event webhook.HookEventType) (git.Entries, []map[string]*model.Job, error) {
	tree, err := commit.SubTree(".github/workflows")
	if _, ok := err.(git.ErrNotExist); ok {
		tree, err = commit.SubTree(".gitea/workflows")
	}
	if _, ok := err.(git.ErrNotExist); ok {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	entries, err := tree.ListEntriesRecursive()
	if err != nil {
		return nil, nil, err
	}

	matchedEntries := make(git.Entries, 0, len(entries))
	jobs := make([]map[string]*model.Job, 0, len(entries))

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		f, err := entry.Blob().DataAsync()
		if err != nil {
			return nil, nil, err
		}
		workflow, err := model.ReadWorkflow(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}

		for _, e := range workflow.On() {
			if e == event.Event() {
				matchedEntries = append(matchedEntries, entry)
				jobs = append(jobs, workflow.Jobs)
				break
			}
		}
		f.Close()
	}

	return matchedEntries, jobs, nil
}
