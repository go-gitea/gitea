import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo, apiHeaders, baseUrl} from './utils.ts';

test('assign issue to project and change column', async ({page}) => {
  const repoName = `e2e-issue-project-${Date.now()}`;
  const user = env.GITEA_TEST_E2E_USER;
  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  await page.goto(`/${user}/${repoName}/projects/new`);
  await page.locator('input[name="title"]').fill('Kanban Board');
  await page.getByRole('button', {name: 'Create Project'}).click();

  const projectLink = page.locator('.milestone-list a', {hasText: 'Kanban Board'}).first();
  await expect(projectLink).toBeVisible();
  const href = await projectLink.getAttribute('href');
  const projectID = href!.split('/').pop();

  // columns created via POST because the web UI uses modals that are hard to drive
  for (const title of ['Backlog', 'In Progress', 'Done']) {
    await page.request.post(`${baseUrl()}/${user}/${repoName}/projects/${projectID}/columns/new`, {
      headers: apiHeaders(),
      form: {title},
    });
  }

  const issueResp = await page.request.post(`${baseUrl()}/api/v1/repos/${user}/${repoName}/issues`, {
    headers: apiHeaders(),
    data: {title: 'Test Issue for Column Picker'},
  });
  expect(issueResp.ok()).toBeTruthy();
  const issue = await issueResp.json();

  await page.goto(`/${user}/${repoName}/issues/${issue.number}`);
  await page.locator('.sidebar-project-combo .ui.dropdown').click();
  await Promise.all([
    page.waitForResponse((resp) => resp.url().includes('/issues/projects') && resp.status() === 200),
    page.locator('.sidebar-project-combo .menu .item', {hasText: 'Kanban Board'}).click(),
  ]);
  await page.waitForLoadState('load');

  await expect(page.locator('.sidebar-project-card')).toBeVisible();
  await expect(page.locator('.sidebar-project-card-header', {hasText: 'Kanban Board'})).toBeVisible();
  await expect(page.locator('.sidebar-project-column-combo')).toBeVisible();

  const columnCombo = page.locator('.sidebar-project-column-combo');
  await expect(columnCombo).toBeVisible({timeout: 10000});
  await columnCombo.locator('.ui.dropdown').click();

  await Promise.all([
    page.waitForResponse((resp) => resp.url().includes('/issues/projects/column') && resp.status() === 200),
    columnCombo.locator('.menu .item', {hasText: 'In Progress'}).click(),
  ]);
  await page.waitForLoadState('load');

  await expect(page.locator('.sidebar-project-column-combo .sidebar-project-column-text')).toContainText('In Progress');
  await expect(page.locator('.timeline-item', {hasText: 'moved this to In Progress'})).toBeVisible();

  await page.locator('.sidebar-project-column-combo .ui.dropdown').click();
  const checkedItem = page.locator('.sidebar-project-column-combo .menu .item.checked');
  await expect(checkedItem).toContainText('In Progress');

  await apiDeleteRepo(page.request, user, repoName);
});
