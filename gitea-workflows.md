# gitea workflows

Issues collected about Gitea project-board and issue/PR workflows.

Gitea has no per-project workflow or rule model; the only automation is the default column for new or unassigned issues (`models/project/column.go`).

## Issues

- [ ] **Project board does not move cards to Done when an issue is closed**
  - Closing an issue (for example via `Closes #60`) leaves the card in its current column, and closed cards are hidden unless "show closed" is on.
  - Related code: `models/project/column.go`, `routers/web/repo/projects.go`
  - Example: fleetzip/frontend-driver-app#60 (closed by PR #61 via `(#60)` reference)

- [ ] **Project board does not move cards when a PR is merged**
  - Merging a PR leaves its card in its current column instead of moving it (for example to Done).
  - Related code: `models/project/column.go`, `routers/web/repo/projects.go`

- [ ] **Project board does not move cards when an issue is reopened**
  - Reopening an issue leaves the card where it was instead of moving it (for example to In Progress).

- [ ] **Project board has no "item added to project" workflow**
  - New items only land in the default column; there is no way to set a status (for example Todo) when an item is added.

- [ ] **Project board does not react to PR review state**
  - Requesting changes on or approving a PR does not move its card.

- [ ] **Project board does not react to a PR being linked to an issue**
  - Linking a PR to an issue does not move the issue card.

- [ ] **Moving a card to a column does not close the issue**
  - Moving a card to Done leaves the issue open.

- [ ] **Project board cannot auto-add issues and PRs from a repo**
  - Items must be added manually; there is no rule to add them automatically, for example filtered by label, assignee or state.

- [ ] **Project board cannot auto-archive items**
  - There is no rule to archive items matching a filter (for example closed and not updated for two weeks).

- Not checked: whether the API or UI has any card-move triggers beyond manual drag-and-drop

Got the inspiration from GitHub Projects' built-in workflows.
