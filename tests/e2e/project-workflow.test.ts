import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  apiCreateLabel, apiCreateProject, apiCreateRepo, apiDeleteRepo, createProjectColumn, login, logout, randomString,
} from './utils.ts';
import type {Page} from '@playwright/test';

const owner = env.GITEA_TEST_E2E_USER;

const sidebar = (page: Page) => page.getByRole('navigation', {name: 'Default workflows'});
const workflowLink = (page: Page, name: string) => sidebar(page).getByRole('link', {name});
const editor = (page: Page) => page.getByRole('form', {name: /^Item added to project/});
const editorButton = (page: Page, name: string) => editor(page).getByRole('button', {name, exact: true});
const editorField = (page: Page, name: string) => editor(page).getByRole('button', {name});

async function choose(page: Page, field: string, option: string) {
  await editorField(page, field).click();
  await page.getByRole('menuitemradio', {name: option}).click();
}

async function createWorkflowProject(page: Page) {
  const repo = `e2e-project-workflow-${randomString(8)}`;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repo, autoInit: false})]);
  const id = await apiCreateProject(page.request, owner, repo, 'Workflows');
  await Promise.all(['Backlog', 'Done'].map((title) => createProjectColumn(page.request, owner, repo, String(id), title)));
  return {repo, url: `/${owner}/${repo}/projects/${id}/workflows`};
}

async function configureItemAdded(page: Page, column: string) {
  await workflowLink(page, 'Item added to project').click();
  await choose(page, 'Move to column', column);
  await editorButton(page, 'Save').click();
}

test('project workflow: configure, toggle, edit and view read-only', async ({page}) => {
  const {repo, url} = await createWorkflowProject(page);
  try {
    await page.goto(url);
    await expect(sidebar(page).getByRole('link')).toHaveCount(8);
    await choose(page, 'Apply to', 'Issues only');
    await configureItemAdded(page, 'Backlog');
    await expect(page).toHaveURL(/\/workflows\/\d+$/);
    await expect(editor(page).getByLabel('Apply to')).toHaveText('Issues only');
    await expect(workflowLink(page, 'Item added to project')).toContainText('(Issues only)');

    await editorButton(page, 'Disable').click();
    await expect(workflowLink(page, 'Item added to project').getByRole('img', {name: 'Disabled'})).toBeVisible();
    await editorButton(page, 'Enable').click();
    await expect(workflowLink(page, 'Item added to project').getByRole('img', {name: 'Enabled'})).toBeVisible();

    await page.goto(`${url}?event=item_added_to_project`);
    await expect(editor(page).getByLabel('Move to column')).toHaveText('Backlog');

    await editorButton(page, 'Edit').click();
    await expect(editorField(page, 'Apply to')).toHaveText('Issues only');
    await choose(page, 'Move to column', 'Done');
    await editorButton(page, 'Save').click();
    await expect(editorButton(page, 'Edit')).toBeVisible();

    await logout(page);
    await page.goto(url);
    await expect(editor(page).getByLabel('Move to column')).toHaveText('Done');
    await expect(editor(page).getByRole('button')).toHaveCount(0);
  } finally {
    await apiDeleteRepo(page.request, owner, repo);
  }
});

test('project workflow: clone with labels and delete', async ({page}) => {
  const {repo, url} = await createWorkflowProject(page);
  try {
    await apiCreateLabel(page.request, owner, repo, 'bug', 'ee0701');
    await page.goto(url);
    await configureItemAdded(page, 'Backlog');

    await editorButton(page, 'Clone').click();
    await editorField(page, 'Add labels').click();
    await page.getByRole('menuitemcheckbox', {name: 'bug'}).click();
    await editorButton(page, 'Save').click();
    await expect(editor(page).getByRole('group', {name: 'Add labels'})).toHaveText('bug');
    await expect(workflowLink(page, 'Item added to project #2')).toHaveAttribute('aria-current', 'page');

    await editorButton(page, 'Edit').click();
    await editorButton(page, 'Remove').click();
    await page.getByRole('button', {name: 'Confirm', exact: true}).click();
    await expect(workflowLink(page, 'Item added to project')).toHaveCount(1);
    await expect(editor(page).getByLabel('Move to column')).toHaveText('Backlog');
  } finally {
    await apiDeleteRepo(page.request, owner, repo);
  }
});
