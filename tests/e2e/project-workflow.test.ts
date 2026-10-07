// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateProject, apiCreateRepo, apiDeleteRepo, createProjectColumn, login, logout, randomString} from './utils.ts';
import type {Page} from '@playwright/test';

const owner = env.GITEA_TEST_E2E_USER;

const sidebar = (page: Page) => page.getByRole('navigation', {name: 'Default workflows'});
const workflowLink = (page: Page, name: string) => sidebar(page).getByRole('link', {name});
const editor = (page: Page) => page.getByRole('form', {name: /^Item opened/});
const editorButton = (page: Page, name: string) => editor(page).getByRole('button', {name, exact: true});

async function openWorkflowsPage(page: Page) {
  const repo = `e2e-project-workflow-${randomString(8)}`;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repo, autoInit: false})]);
  const projectId = await apiCreateProject(page.request, owner, repo, 'Workflows');
  await Promise.all(['Backlog', 'Done'].map((title) => createProjectColumn(page.request, owner, repo, String(projectId), title)));
  const url = `/${owner}/${repo}/projects/${projectId}/workflows`;
  await page.goto(url);
  await expect(sidebar(page)).toBeVisible();
  return {repo, url};
}

async function configureItemOpened(page: Page, column: string) {
  await workflowLink(page, 'Item opened').click();
  await expect(workflowLink(page, 'Item opened')).toHaveAttribute('aria-current', 'page');
  await editor(page).getByLabel('Move to column').selectOption({label: column});
  await editorButton(page, 'Save').click();
  await expect(editorButton(page, 'Edit')).toBeVisible();
}

test('project workflow: configure, toggle and edit', async ({page}) => {
  const {repo, url} = await openWorkflowsPage(page);
  try {
    await expect(sidebar(page).getByRole('link')).toHaveCount(9);
    await workflowLink(page, 'Item opened').click();
    await editor(page).getByLabel('Apply to').selectOption({label: 'Issues only'});
    await configureItemOpened(page, 'Backlog');
    await expect(page).toHaveURL(/\/workflows\/\d+$/);
    await expect(editor(page).getByLabel('Apply to')).toHaveText('Issues only');
    await expect(editor(page).getByLabel('Move to column')).toHaveText('Backlog');
    await expect(workflowLink(page, 'Item opened')).toContainText('(Issues only)');
    await expect(workflowLink(page, 'Item opened').getByRole('img', {name: 'Enabled'})).toBeVisible();

    await editorButton(page, 'Disable').click();
    await expect(editor(page).getByText('Disabled', {exact: true})).toBeVisible();
    await expect(workflowLink(page, 'Item opened').getByRole('img', {name: 'Disabled'})).toBeVisible();
    await editorButton(page, 'Enable').click();
    await expect(editor(page).getByText('Enabled', {exact: true})).toBeVisible();

    await page.goto(`${url}/item_opened`);
    await expect(workflowLink(page, 'Item opened')).toHaveAttribute('aria-current', 'page');
    await expect(editor(page).getByLabel('Move to column')).toHaveText('Backlog');

    await editorButton(page, 'Edit').click();
    await expect(editor(page).getByLabel('Apply to')).toHaveValue('issue');
    await editor(page).getByLabel('Move to column').selectOption({label: 'Done'});
    await editorButton(page, 'Save').click();
    await expect(editor(page).getByLabel('Move to column')).toHaveText('Done');
  } finally {
    await apiDeleteRepo(page.request, owner, repo);
  }
});

test('project workflow: clone, cancel and delete', async ({page}) => {
  const {repo} = await openWorkflowsPage(page);
  try {
    await configureItemOpened(page, 'Backlog');

    await editorButton(page, 'Clone').click();
    await expect(workflowLink(page, 'Item opened #2')).toHaveAttribute('aria-current', 'page');
    await editorButton(page, 'Cancel').click();
    await expect(workflowLink(page, 'Item opened')).toHaveCount(1);
    await expect(editorButton(page, 'Edit')).toBeVisible();

    await editorButton(page, 'Clone').click();
    await editor(page).getByLabel('Move to column').selectOption({label: 'Done'});
    await editorButton(page, 'Save').click();
    await expect(workflowLink(page, 'Item opened')).toHaveCount(2);
    await expect(workflowLink(page, 'Item opened #2')).toHaveAttribute('aria-current', 'page');
    await expect(editor(page).getByLabel('Move to column')).toHaveText('Done');

    await editorButton(page, 'Edit').click();
    await editorButton(page, 'Remove').click();
    await page.getByRole('button', {name: 'Confirm', exact: true}).click();
    await expect(workflowLink(page, 'Item opened')).toHaveCount(1);
    await expect(workflowLink(page, 'Item opened')).toHaveAttribute('aria-current', 'page');
    await expect(editor(page).getByLabel('Move to column')).toHaveText('Backlog');
  } finally {
    await apiDeleteRepo(page.request, owner, repo);
  }
});

test('project workflow: anonymous users get a read-only view', async ({page}) => {
  const {repo, url} = await openWorkflowsPage(page);
  try {
    await configureItemOpened(page, 'Backlog');
    await logout(page);
    await page.goto(url);
    await expect(workflowLink(page, 'Item opened')).toHaveAttribute('aria-current', 'page');
    await expect(editor(page).getByLabel('Move to column')).toHaveText('Backlog');
    await expect(editor(page).getByRole('button')).toHaveCount(0);
  } finally {
    await apiDeleteRepo(page.request, owner, repo);
  }
});
