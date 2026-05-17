// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo, createProject, createProjectColumn, randomString} from './utils.ts';
import type {Page} from '@playwright/test';

// ── helpers ─────────────────────────────────────────────────────────────────

/** Create a minimal project + two columns and navigate to its workflows page. */
async function setupWorkflowPage(page: Page, repoName: string) {
  const user = env.GITEA_TEST_E2E_USER;
  const project = await createProject(page, {owner: user, repo: repoName, title: 'WF Project'});
  await Promise.all([
    createProjectColumn(page.request, user, repoName, String(project.id), 'Backlog'),
    createProjectColumn(page.request, user, repoName, String(project.id), 'Done'),
  ]);
  await page.goto(`/${user}/${repoName}/projects/${project.id}/workflows`);
  await expect(page.locator('.workflow-sidebar')).toBeVisible();
  return project;
}

/** Click the first sidebar item and save it with the first column option. */
async function configureFirstWorkflow(page: Page) {
  const firstItem = page.locator('.workflow-item').first();
  await firstItem.click();
  await expect(editorActionButton(page, 'Save')).toBeVisible();
  // Use the "Move to column" action field specifically; the first select in the form
  // is "Apply to" (issue-type filter), not the column action select.
  await moveToColumnSelect(page).selectOption({index: 1});
  await clickEditorAction(page, 'Save');
  await expect(page.locator('.workflow-editor .workflow-status.status-enabled')).toBeVisible();
}

/** Returns the "Move to column" action select inside the workflow editor. */
function moveToColumnSelect(page: Page) {
  return page.locator('.workflow-editor .field').filter({hasText: 'Move to column'}).locator('select');
}

/** Returns the "Apply to" filter select inside the workflow editor. */
function applyToSelect(page: Page) {
  return page.locator('.workflow-editor .field').filter({hasText: 'Apply to'}).locator('select');
}

function editorActionButton(page: Page, text: string) {
  return page.locator('.editor-actions-header button', {hasText: text});
}

async function clickEditorAction(page: Page, text: string) {
  const button = editorActionButton(page, text);
  await expect(button).toBeVisible();
  await button.click();
  // Edit/Clone use deferred emits (setTimeout), so wait for the mode switch to complete.
  if (text === 'Edit' || text === 'Cancel') {
    await expect(editorActionButton(page, text)).toBeHidden();
  } else if (text === 'Clone') {
    // Clone switches to edit mode; wait for Cancel button to confirm.
    await expect(editorActionButton(page, 'Cancel')).toBeVisible();
  }
}

function moveToColumnReadonlyValue(page: Page) {
  return page.locator('.workflow-editor .field').filter({hasText: 'Move to column'}).locator('.readonly-value');
}

test('project workflow: configure and toggle enable/disable', async ({page}) => {
  const repoName = `e2e-workflow-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  test.setTimeout(30000);

  await Promise.all([
    login(page),
    apiCreateRepo(page.request, {name: repoName}),
  ]);

  try {
    const project = await createProject(page, {owner: user, repo: repoName, title: 'Workflow Project'});
    await Promise.all([
      createProjectColumn(page.request, user, repoName, String(project.id), 'Backlog'),
      createProjectColumn(page.request, user, repoName, String(project.id), 'Done'),
    ]);

    await page.goto(`/${user}/${repoName}/projects/${project.id}/workflows`);

    // Sidebar and first workflow item should be visible after Vue mounts
    const sidebar = page.locator('.workflow-sidebar');
    await expect(sidebar).toBeVisible();
    const firstItem = page.locator('.workflow-item').first();
    await expect(firstItem).toBeVisible();

    // Click the first workflow; unconfigured events auto-enter edit mode
    await firstItem.click();
    const editor = page.locator('.workflow-editor');
    await expect(editor).toBeVisible();

    // Save button visible means we are in edit mode
    const saveBtn = editorActionButton(page, 'Save');
    await expect(saveBtn).toBeVisible();

    // Select the "Done" column in the "Move to column" action select
    await moveToColumnSelect(page).selectOption({label: 'Done'});

    // Save the workflow configuration
    await saveBtn.click();

    // After save, view mode is active and status badge shows "Enabled"
    await expect(editor.locator('.workflow-status.status-enabled')).toBeVisible();
    await expect(editor.locator('.editor-actions-header button', {hasText: 'Edit'})).toBeVisible();

    // Disable the workflow
    await clickEditorAction(page, 'Disable');
    await expect(editor.locator('.workflow-status.status-disabled')).toBeVisible();

    // Re-enable the workflow
    await clickEditorAction(page, 'Enable');
    await expect(editor.locator('.workflow-status.status-enabled')).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});

// ── new tests ────────────────────────────────────────────────────────────────

test('project workflow: sidebar lists all 9 event types with inactive dots', async ({page}) => {
  const repoName = `e2e-wf-sidebar-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  test.setTimeout(30000);
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  try {
    const project = await createProject(page, {owner: user, repo: repoName, title: 'Sidebar Test'});
    await page.goto(`/${user}/${repoName}/projects/${project.id}/workflows`);
    await expect(page.locator('.workflow-sidebar')).toBeVisible();

    // All 9 event types must be visible and each should start with an inactive dot.
    const items = page.locator('.workflow-item');
    await expect(items).toHaveCount(9);
    const inactiveDots = page.locator('.workflow-item .status-inactive');
    await expect(inactiveDots).toHaveCount(9);
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});

test('project workflow: status dot colour changes on configure / disable', async ({page}) => {
  const repoName = `e2e-wf-dot-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  test.setTimeout(30000);
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  try {
    await setupWorkflowPage(page, repoName);

    // Before configuration: first item dot is inactive (grey).
    await expect(page.locator('.workflow-item').first().locator('.status-inactive')).toBeVisible();

    // Configure the first workflow.
    await configureFirstWorkflow(page);

    // After save the first item's dot must switch to active (green).
    await expect(page.locator('.workflow-item').first().locator('.status-active')).toBeVisible();
    // All other items remain inactive.
    await expect(page.locator('.workflow-item .status-inactive')).toHaveCount(8);

    // Disable the workflow — dot becomes disabled (red).
    await clickEditorAction(page, 'Disable');
    await expect(page.locator('.workflow-item').first().locator('.status-disabled')).toBeVisible();

    // Re-enable — back to active (green).
    await clickEditorAction(page, 'Enable');
    await expect(page.locator('.workflow-item').first().locator('.status-active')).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});

test('project workflow: cancel clone removes pending clone and restores original', async ({page}) => {
  const repoName = `e2e-wf-cancel-clone-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  test.setTimeout(30000);
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  try {
    await setupWorkflowPage(page, repoName);
    await configureFirstWorkflow(page);

    // The configured workflow is now shown in view mode.
    await expect(page.locator('.editor-actions-header button', {hasText: 'Edit'})).toBeVisible();

    // Clone it — a new (10th) sidebar entry appears and we enter edit mode.
    await clickEditorAction(page, 'Clone');
    await expect(page.locator('.workflow-item')).toHaveCount(10);
    await expect(editorActionButton(page, 'Save')).toBeVisible();

    // Cancel the clone — the pending entry must be removed.
    await clickEditorAction(page, 'Cancel');
    await expect(page.locator('.workflow-item')).toHaveCount(9);

    // The original workflow should be selected (active) and in view mode.
    await expect(page.locator('.workflow-item').first()).toHaveClass(/active/);
    await expect(page.locator('.editor-actions-header button', {hasText: 'Edit'})).toBeVisible();
    await expect(page.locator('.editor-actions-header button', {hasText: 'Save'})).toBeHidden();
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});

test('project workflow: saving without any action shows validation error', async ({page}) => {
  const repoName = `e2e-wf-validate-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  test.setTimeout(30000);
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  try {
    await setupWorkflowPage(page, repoName);

    // Click an unconfigured workflow — it auto-enters edit mode.
    await page.locator('.workflow-item').first().click();
    await expect(page.locator('.editor-actions-header button', {hasText: 'Save'})).toBeVisible();

    // Deliberately leave all selects at their default empty values, then save.
    await clickEditorAction(page, 'Save');

    // A Toastify error notification must appear containing the validation text.
    // Note: .toast-body always has a hidden "1" span prefix, so we use a regex.
    await expect(page.locator('.toastify.on .toast-body')).toContainText(/at least one action/i);

    // The editor must remain in edit mode (not have been navigated away).
    await expect(page.locator('.editor-actions-header button', {hasText: 'Save'})).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});

test('project workflow: "Apply to" filter persists across save and re-open', async ({page}) => {
  const repoName = `e2e-wf-filter-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  test.setTimeout(30000);
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  try {
    await setupWorkflowPage(page, repoName);

    // "Item opened" (first item) supports the issue-type filter and column action.
    await page.locator('.workflow-item').first().click();
    await expect(page.locator('.editor-actions-header button', {hasText: 'Save'})).toBeVisible();

    // Set "Apply to" → "Issues only".
    await applyToSelect(page).selectOption({label: 'Issues only'});

    // Set the required column action too.
    await moveToColumnSelect(page).selectOption({index: 1});
    await clickEditorAction(page, 'Save');
    await expect(page.locator('.workflow-editor .workflow-status.status-enabled')).toBeVisible();

    // Re-open in edit mode and verify the saved "Apply to" value is restored.
    await clickEditorAction(page, 'Edit');
    await expect(applyToSelect(page)).toHaveValue('issue');
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});

test('project workflow: editing a saved workflow updates its configuration', async ({page}) => {
  const repoName = `e2e-wf-edit-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  test.setTimeout(30000);
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  try {
    await setupWorkflowPage(page, repoName);
    await configureFirstWorkflow(page);  // saves with 'Backlog' (index 1)

    // Edit the workflow and switch to the second column ('Done', index 2).
    await clickEditorAction(page, 'Edit');
    await moveToColumnSelect(page).selectOption({index: 2});
    await clickEditorAction(page, 'Save');

    // After save, view mode should reflect the updated column title.
    await expect(page.locator('.workflow-editor .workflow-status.status-enabled')).toBeVisible();
    await expect(moveToColumnReadonlyValue(page)).toContainText('Done');
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});

test('project workflow: direct URL navigation selects the correct workflow', async ({page}) => {
  const repoName = `e2e-wf-url-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  test.setTimeout(30000);
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  try {
    const project = await setupWorkflowPage(page, repoName);
    await configureFirstWorkflow(page);

    // Capture the URL that was set after save (contains the numeric workflow ID).
    const savedUrl = page.url();

    // Navigate away then back via the saved URL.
    await page.goto(`/${user}/${repoName}/projects/${project.id}/workflows`);
    await expect(page.locator('.workflow-sidebar')).toBeVisible();
    await page.goto(savedUrl);

    // The saved workflow should be pre-selected and in view mode.
    await expect(page.locator('.workflow-item.active')).toBeVisible();
    await expect(page.locator('.workflow-editor .workflow-status.status-enabled')).toBeVisible();
    await expect(page.locator('.editor-actions-header button', {hasText: 'Edit'})).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});

test('project workflow: clone and delete', async ({page}) => {
  const repoName = `e2e-workflow-clone-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  test.setTimeout(30000);

  await Promise.all([
    login(page),
    apiCreateRepo(page.request, {name: repoName}),
  ]);

  try {
    const project = await createProject(page, {owner: user, repo: repoName, title: 'Clone Workflow Project'});
    await createProjectColumn(page.request, user, repoName, String(project.id), 'In Progress');

    await page.goto(`/${user}/${repoName}/projects/${project.id}/workflows`);

    const firstItem = page.locator('.workflow-item').first();
    await expect(firstItem).toBeVisible();
    await firstItem.click();

    const editor = page.locator('.workflow-editor');
    const saveBtn = editorActionButton(page, 'Save');
    await expect(saveBtn).toBeVisible();

    // Configure the workflow: pick a column and save
    await moveToColumnSelect(page).selectOption({label: 'In Progress'});
    await saveBtn.click();
    await expect(editor.locator('.workflow-status.status-enabled')).toBeVisible();

    // Verify the sidebar now shows all 9 event types
    await expect(page.locator('.workflow-item')).toHaveCount(9);

    // Clone the configured workflow
    await clickEditorAction(page, 'Clone');
    // A new entry for the same event type appears in the sidebar
    await expect(page.locator('.workflow-item')).toHaveCount(10);

    // Save the clone (pre-filled from the original)
    await clickEditorAction(page, 'Save');
    await expect(editor.locator('.workflow-status.status-enabled')).toBeVisible();

    // Delete the cloned workflow
    await clickEditorAction(page, 'Edit');
    await clickEditorAction(page, 'Delete');

    // Confirm deletion in the modal
    await page.locator('.ui.g-modal-confirm .ui.red.ok.button').click();

    // Back to 9 items
    await expect(page.locator('.workflow-item')).toHaveCount(9);
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});
