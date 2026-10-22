// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package project manages project boards and their workflows.
//
// Workflow permissions are checked when workflows are managed, not when they run:
//   - Viewing a project's workflows needs read access to the project.
//   - Managing them needs write access to the project plus authority over every item it can hold, see
//     CanManageWorkflows: write access to both issues and pull requests of a repository project's repository,
//     being an owner of an organization project's organization or a site administrator, or being the user of a
//     user project. Tokens limited to public repositories can't manage owner-level projects' workflows.
//   - A workflow runs when its event happens to an item of an open project that can hold the item's repository,
//     unless that repository is archived, no matter who caused the event. Its actions apply without further
//     permission checks, including labeling and closing pull requests. Pull requests are never reopened.
//   - Changes made by a workflow show as made by "Project workflow" on behalf of the user who triggered the event,
//     and don't trigger other workflows.
//   - Only users who can read an item can put it on a board or move its card, so a workflow never acts for
//     someone on an item they can't see.
package project
