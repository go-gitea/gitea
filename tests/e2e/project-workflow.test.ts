// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo, createProject, createProjectColumn, randomString} from './utils.ts';

test('project workflow: configure and toggle enable/disable', async ({page}) => {
  const repoName = `e2e-workflow-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;

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
    const saveBtn = page.locator('.editor-actions-header button', {hasText: 'Save'});
    await expect(saveBtn).toBeVisible();

    // Select the "Done" column in the "Move to column" action select
    const columnSelect = editor.locator('select').first();
    await columnSelect.selectOption({label: 'Done'});

    // Save the workflow configuration
    await saveBtn.click();

    // After save, view mode is active and status badge shows "Enabled"
    await expect(editor.locator('.workflow-status.status-enabled')).toBeVisible();
    await expect(editor.locator('.editor-actions-header button', {hasText: 'Edit'})).toBeVisible();

    // Disable the workflow
    await page.locator('.editor-actions-header button', {hasText: 'Disable'}).click();
    await expect(editor.locator('.workflow-status.status-disabled')).toBeVisible();

    // Re-enable the workflow
    await page.locator('.editor-actions-header button', {hasText: 'Enable'}).click();
    await expect(editor.locator('.workflow-status.status-enabled')).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});

test('project workflow: clone and delete', async ({page}) => {
  const repoName = `e2e-workflow-clone-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;

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
    const saveBtn = page.locator('.editor-actions-header button', {hasText: 'Save'});
    await expect(saveBtn).toBeVisible();

    // Configure the workflow: pick a column and save
    await editor.locator('select').first().selectOption({label: 'In Progress'});
    await saveBtn.click();
    await expect(editor.locator('.workflow-status.status-enabled')).toBeVisible();

    // Verify the sidebar now shows all 9 event types
    await expect(page.locator('.workflow-item')).toHaveCount(9);

    // Clone the configured workflow
    await page.locator('.editor-actions-header button', {hasText: 'Clone'}).click();
    // A new entry for the same event type appears in the sidebar
    await expect(page.locator('.workflow-item')).toHaveCount(10);

    // Save the clone (pre-filled from the original)
    await page.locator('.editor-actions-header button', {hasText: 'Save'}).click();
    await expect(editor.locator('.workflow-status.status-enabled')).toBeVisible();

    // Delete the cloned workflow
    await page.locator('.editor-actions-header button', {hasText: 'Edit'}).click();
    await page.locator('.editor-actions-header button', {hasText: 'Delete'}).click();

    // Confirm deletion in the modal
    await page.locator('.ui.g-modal-confirm .ui.red.ok.button').click();

    // Back to 9 items
    await expect(page.locator('.workflow-item')).toHaveCount(9);
  } finally {
    await apiDeleteRepo(page.request, user, repoName);
  }
});
