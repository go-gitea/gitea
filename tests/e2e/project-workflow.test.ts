// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  apiAddCollaborator,
  apiCreateRepo,
  apiCreateUser,
  apiDeleteRepo,
  apiDeleteUser,
  createProject,
  createProjectColumn,
  login,
  loginUser,
  logout,
  randomString,
} from './utils.ts';
import type {Page} from '@playwright/test';

// ── locators ────────────────────────────────────────────────────────────────
//
// The workflow DOM is fully labelled, so everything here is addressed by role or
// by label. Two exceptions are deliberate and noted where they appear: the sidebar
// status dots (a colour with no textual or semantic equivalent) and the toast body
// (shared toast markup exposes no role).

const testUser = () => env.GITEA_TEST_E2E_USER;

/** The workflow list. `<nav aria-label="Default Workflows">` holding one link per row. */
function sidebar(page: Page) {
  return page.getByRole('navigation', {name: 'Default Workflows'});
}

/**
 * A sidebar row by its visible name. Accessible-name matching is a substring
 * match, so 'Item opened' also finds the numbered 'Item opened #2' that appears
 * once an event type has more than one workflow.
 */
function workflowLink(page: Page, name: string) {
  return sidebar(page).getByRole('link', {name});
}

/**
 * A button in the workflow editor. Scoped to the editor because the surrounding
 * repository page has its own Edit and Cancel buttons; without the scope, asserting
 * that a reader sees no Edit button would match one of those instead.
 */
function editorButton(page: Page, name: string) {
  return page.locator('.workflow-editor').getByRole('button', {name, exact: true});
}

/** The OK button of the shared confirmation modal, which renders outside the editor. */
function confirmButton(page: Page) {
  return page.getByRole('button', {name: 'Confirm', exact: true});
}

/**
 * The Enabled/Disabled badge. Scoped to the editor because the badge is a bare
 * span; everything queried inside that scope is semantic.
 */
function statusBadge(page: Page) {
  return page.locator('.workflow-editor').getByText(/^(Enabled|Disabled)$/);
}

/** The status dot of a sidebar row: a colour, so it has no semantic equivalent. */
function statusDot(page: Page, name: string, state: 'active' | 'inactive' | 'disabled') {
  return workflowLink(page, name).locator(`.status-${state}`);
}

// ── helpers ─────────────────────────────────────────────────────────────────

/** Create a project with two columns and open its workflows page. */
async function openWorkflowsPage(page: Page, repoName: string) {
  const owner = testUser();
  const project = await createProject(page, {owner, repo: repoName, title: 'WF Project'});
  // Created sequentially so the column option order stays deterministic.
  await createProjectColumn(page.request, owner, repoName, String(project.id), 'Backlog');
  await createProjectColumn(page.request, owner, repoName, String(project.id), 'Done');
  await page.goto(`/${owner}/${repoName}/projects/${project.id}/workflows`);
  await expect(sidebar(page)).toBeVisible();
  return project;
}

/**
 * Click a sidebar row and wait for it to become the selection.
 *
 * The sidebar debounces selection, and the page auto-selects the first row on
 * mount, so acting on the editor straight after a click can silently operate on
 * the previously selected workflow. aria-current is the signal that the click has
 * landed: it is derived from the resolved selection, so it cannot be set while the
 * editor pane is still empty.
 */
async function selectWorkflow(page: Page, name: string) {
  await workflowLink(page, name).click();
  await expect(workflowLink(page, name)).toHaveAttribute('aria-current', 'page');
}

/** Select a workflow, give it a column action and save it. */
async function configureWorkflow(page: Page, name: string, column: string) {
  await selectWorkflow(page, name);
  // An unconfigured row opens straight into edit mode.
  await expect(editorButton(page, 'Save')).toBeVisible();
  await page.getByLabel('Move to column').selectOption({label: column});
  await saveWorkflow(page);
}

async function saveWorkflow(page: Page) {
  await editorButton(page, 'Save').click();
  await expect(editorButton(page, 'Edit')).toBeVisible();
}

/** Clone the selected workflow and wait for the pending copy to open in edit mode. */
async function cloneSelected(page: Page) {
  await editorButton(page, 'Clone').click();
  await expect(editorButton(page, 'Cancel')).toBeVisible();
  await expect(editorButton(page, 'Save')).toBeVisible();
}

async function enterEditMode(page: Page) {
  await editorButton(page, 'Edit').click();
  await expect(editorButton(page, 'Save')).toBeVisible();
}

// ── tests ───────────────────────────────────────────────────────────────────

test('project workflow: configure, disable and re-enable', async ({page}) => {
  const repoName = `e2e-workflow-${randomString(8)}`;
  const owner = testUser();

  await Promise.all([
    login(page),
    apiCreateRepo(page.request, {name: repoName, autoInit: false}),
  ]);

  try {
    await openWorkflowsPage(page, repoName);

    // Every event type is offered, and nothing is configured yet.
    await expect(sidebar(page).getByRole('link')).toHaveCount(9);
    await expect(statusDot(page, 'Item opened', 'inactive')).toBeVisible();

    await configureWorkflow(page, 'Item opened', 'Done');
    await expect(statusBadge(page)).toHaveText('Enabled');
    await expect(statusDot(page, 'Item opened', 'active')).toBeVisible();

    await editorButton(page, 'Disable').click();
    await expect(statusBadge(page)).toHaveText('Disabled');
    await expect(statusDot(page, 'Item opened', 'disabled')).toBeVisible();

    await editorButton(page, 'Enable').click();
    await expect(statusBadge(page)).toHaveText('Enabled');
    await expect(statusDot(page, 'Item opened', 'active')).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, owner, repoName);
  }
});

test('project workflow: filter and action survive save, reload and edit', async ({page}) => {
  const repoName = `e2e-wf-filter-${randomString(8)}`;
  const owner = testUser();

  await Promise.all([
    login(page),
    apiCreateRepo(page.request, {name: repoName, autoInit: false}),
  ]);

  try {
    const project = await openWorkflowsPage(page, repoName);

    await selectWorkflow(page, 'Item opened');
    await expect(editorButton(page, 'Save')).toBeVisible();
    await page.getByLabel('Apply to').selectOption({label: 'Issues only'});
    await page.getByLabel('Move to column').selectOption({label: 'Backlog'});
    await editorButton(page, 'Save').click();
    await expect(editorButton(page, 'Edit')).toBeVisible();

    // View mode reads back what was saved.
    await expect(page.getByLabel('Apply to')).toHaveText('Issues only');
    await expect(page.getByLabel('Move to column')).toHaveText('Backlog');

    // A full reload proves the values came from the server, not from local state.
    await page.goto(`/${owner}/${repoName}/projects/${project.id}/workflows`);
    await expect(sidebar(page)).toBeVisible();
    await selectWorkflow(page, 'Item opened');
    await expect(page.getByLabel('Apply to')).toHaveText('Issues only');

    // Editing the saved workflow updates it.
    await enterEditMode(page);
    await expect(page.getByLabel('Apply to')).toHaveValue('issue');
    await page.getByLabel('Move to column').selectOption({label: 'Done'});
    await saveWorkflow(page);
    await expect(page.getByLabel('Move to column')).toHaveText('Done');
  } finally {
    await apiDeleteRepo(page.request, owner, repoName);
  }
});

test('project workflow: saving with no action surfaces the server error', async ({page}) => {
  const repoName = `e2e-wf-validate-${randomString(8)}`;
  const owner = testUser();

  await Promise.all([
    login(page),
    apiCreateRepo(page.request, {name: repoName, autoInit: false}),
  ]);

  try {
    await openWorkflowsPage(page, repoName);

    await selectWorkflow(page, 'Item opened');
    await expect(editorButton(page, 'Save')).toBeVisible();

    // Leave every field at its default and save, so the server rejects it.
    await editorButton(page, 'Save').click();

    // Toasts carry no role, so the body is addressed by class. useInnerText skips
    // the hidden duplicate-count span that the shared toast markup always emits.
    await expect(page.locator('.toastify.on .toast-body'))
      .toHaveText('At least one action must be configured', {useInnerText: true});

    // The editor stays in edit mode with the rejected input still on screen.
    await expect(editorButton(page, 'Save')).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, owner, repoName);
  }
});

test('project workflow: cancelling a clone restores the original', async ({page}) => {
  const repoName = `e2e-wf-cancel-clone-${randomString(8)}`;
  const owner = testUser();

  await Promise.all([
    login(page),
    apiCreateRepo(page.request, {name: repoName, autoInit: false}),
  ]);

  try {
    await openWorkflowsPage(page, repoName);
    await configureWorkflow(page, 'Item opened', 'Backlog');

    await cloneSelected(page);
    await expect(sidebar(page).getByRole('link')).toHaveCount(10);

    await editorButton(page, 'Cancel').click();

    // The pending row is gone and the original is selected again, in view mode.
    await expect(sidebar(page).getByRole('link')).toHaveCount(9);
    await expect(workflowLink(page, 'Item opened')).toHaveAttribute('aria-current', 'page');
    await expect(editorButton(page, 'Edit')).toBeVisible();
    await expect(editorButton(page, 'Save')).toHaveCount(0);
  } finally {
    await apiDeleteRepo(page.request, owner, repoName);
  }
});

test('project workflow: a deep link selects the workflow it names', async ({page}) => {
  const repoName = `e2e-wf-url-${randomString(8)}`;
  const owner = testUser();

  await Promise.all([
    login(page),
    apiCreateRepo(page.request, {name: repoName, autoInit: false}),
  ]);

  try {
    const project = await openWorkflowsPage(page, repoName);
    await configureWorkflow(page, 'Item closed', 'Done');

    // Saving rewrites the URL to address the workflow by its new id.
    const savedUrl = page.url();
    expect(savedUrl).toMatch(/\/workflows\/\d+$/);

    // Leave, come back through the deep link, and the named workflow is selected.
    await page.goto(`/${owner}/${repoName}/projects/${project.id}/workflows`);
    await expect(sidebar(page)).toBeVisible();
    await page.goto(savedUrl);

    await expect(workflowLink(page, 'Item closed')).toHaveAttribute('aria-current', 'page');
    await expect(statusBadge(page)).toHaveText('Enabled');
    await expect(page.getByLabel('Move to column')).toHaveText('Done');
  } finally {
    await apiDeleteRepo(page.request, owner, repoName);
  }
});

test('project workflow: two pending clones of one event type stay independent', async ({page}) => {
  const repoName = `e2e-wf-clone-${randomString(8)}`;
  const owner = testUser();
  // The longest flow in this file: six server round-trips plus a confirm modal.
  test.setTimeout(20000);

  await Promise.all([
    login(page),
    apiCreateRepo(page.request, {name: repoName, autoInit: false}),
  ]);

  try {
    await openWorkflowsPage(page, repoName);

    // Two saved workflows of the same event type, the second cloned from the first.
    await configureWorkflow(page, 'Item opened', 'Backlog');
    await cloneSelected(page);
    await saveWorkflow(page);
    await expect(workflowLink(page, 'Item opened')).toHaveCount(2);

    // Clone each of them, leaving two pending clones of the same event type.
    await selectWorkflow(page, 'Item opened #1');
    await cloneSelected(page);
    await expect(workflowLink(page, 'Item opened')).toHaveCount(3);

    await selectWorkflow(page, 'Item opened #2');
    await cloneSelected(page);
    await expect(workflowLink(page, 'Item opened')).toHaveCount(4);

    // Each row is addressable in its own right: the clones must not share a key,
    // and clicking one must select that one rather than the first of its type.
    await selectWorkflow(page, 'Item opened #3');
    await expect(workflowLink(page, 'Item opened #3')).toHaveAttribute('aria-current', 'page');
    await expect(workflowLink(page, 'Item opened #4')).not.toHaveAttribute('aria-current', 'page');
    await expect(editorButton(page, 'Save')).toBeVisible();

    // Saving one pending clone reloads the list; the other must survive it.
    await page.getByLabel('Move to column').selectOption({label: 'Done'});
    await saveWorkflow(page);
    await expect(workflowLink(page, 'Item opened')).toHaveCount(4);

    // Deleting takes the row away again, through the confirmation modal.
    await enterEditMode(page);
    await editorButton(page, 'Delete').click();
    await confirmButton(page).click();
    await expect(workflowLink(page, 'Item opened')).toHaveCount(3);
  } finally {
    await apiDeleteRepo(page.request, owner, repoName);
  }
});

test('project workflow: a project reader sees the page read-only', async ({page}) => {
  const repoName = `e2e-wf-reader-${randomString(8)}`;
  const readerName = `e2e-wf-reader-${randomString(8)}`;
  const owner = testUser();

  await Promise.all([
    apiCreateRepo(page.request, {name: repoName, autoInit: false}),
    apiCreateUser(page.request, readerName),
  ]);

  try {
    // Configure a workflow as the owner, then hand the repo to a read-only user.
    await login(page);
    const project = await openWorkflowsPage(page, repoName);
    await configureWorkflow(page, 'Item opened', 'Backlog');
    await apiAddCollaborator(page.request, owner, repoName, readerName, 'read');

    // Drop the owner's session first: /user/login on an already-authenticated
    // request redirects without switching user, so the page would stay an owner.
    await logout(page);
    await loginUser(page, readerName);
    await page.goto(`/${owner}/${repoName}/projects/${project.id}/workflows`);
    await expect(sidebar(page)).toBeVisible();
    await selectWorkflow(page, 'Item opened');

    // The configuration is readable...
    await expect(page.getByLabel('Move to column')).toHaveText('Backlog');
    // ...and every control that would change it is absent, not merely disabled.
    for (const name of ['Edit', 'Save', 'Cancel', 'Clone', 'Delete', 'Disable', 'Enable']) {
      await expect(editorButton(page, name)).toHaveCount(0);
    }
  } finally {
    await Promise.all([
      apiDeleteRepo(page.request, owner, repoName),
      apiDeleteUser(page.request, readerName),
    ]);
  }
});
