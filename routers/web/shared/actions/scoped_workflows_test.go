// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_module "gitea.dev/modules/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveScopedStatusContexts(t *testing.T) {
	t.Run("jobs x events; job name is its name: or its id", func(t *testing.T) {
		content := []byte(`name: CI
on: [push, pull_request]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - run: echo
  build:
    name: Build It
    runs-on: ubuntu-latest
    steps:
      - run: echo
`)
		events, err := actions_module.GetEventsFromContent(content)
		require.NoError(t, err)
		got := deriveScopedStatusContexts("org/src", "CI", content, events)
		assert.ElementsMatch(t, []string{
			"org/src: CI / lint (push)",
			"org/src: CI / lint (pull_request)",
			"org/src: CI / Build It (push)",
			"org/src: CI / Build It (pull_request)",
		}, got)
	})

	t.Run("only status-producing events; workflow_dispatch/schedule/workflow_call skipped", func(t *testing.T) {
		content := []byte(`name: CI
on:
  push:
  workflow_dispatch:
  workflow_call:
  schedule:
    - cron: "0 0 * * *"
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: echo
`)
		events, err := actions_module.GetEventsFromContent(content)
		require.NoError(t, err)
		got := deriveScopedStatusContexts("org/src", "CI", content, events)
		assert.Equal(t, []string{"org/src: CI / j (push)"}, got) // only push posts a commit status
	})

	t.Run("a workflow_dispatch-only workflow has no expected contexts", func(t *testing.T) {
		content := []byte(`name: Deploy
on: workflow_dispatch
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: echo
`)
		events, err := actions_module.GetEventsFromContent(content)
		require.NoError(t, err)
		got := deriveScopedStatusContexts("org/src", "Deploy", content, events)
		assert.Empty(t, got) // workflow_dispatch posts no commit status -> nothing to preview (and it cannot be a required check)
	})
}
