import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateIssue, createProjectColumn, randomString} from './utils.ts';

test('assign issue to project and change column', async ({page}) => {
  const repoName = `e2e-issue-project-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  await page.goto(`/${user}/${repoName}/projects/new`);
  await page.locator('input[name="title"]').fill('Kanban Board');
  await page.getByRole('button', {name: 'Create Project'}).click();
  const projectLink = page.locator('.milestone-list a', {hasText: 'Kanban Board'}).first();
  await expect(projectLink).toBeVisible();
  const href = await projectLink.getAttribute('href');
  const projectID = href!.split('/').pop()!;
  // columns created via POST because the web UI uses modals that are hard to drive
  await Promise.all([
    ...['Backlog', 'In Progress', 'Done'].map((title) => createProjectColumn(page.request, user, repoName, projectID, title)),
    apiCreateIssue(page.request, user, repoName, {title: 'Column picker test'}),
  ]);
  await page.goto(`/${user}/${repoName}/issues/1`);
  await page.locator('.sidebar-project-combo .ui.dropdown').click();
  await page.locator('.sidebar-project-combo .menu a.item', {hasText: 'Kanban Board'}).click();
  const columnCombo = page.locator('.sidebar-project-column-combo');
  await expect(columnCombo).toBeVisible();
  await columnCombo.locator('.ui.dropdown').click();
  await columnCombo.locator('.menu a.item', {hasText: 'In Progress'}).click();
  await expect(columnCombo.getByTestId('sidebar-project-column-text')).toHaveText('In Progress');
});
