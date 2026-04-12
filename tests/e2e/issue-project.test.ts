import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo, apiHeaders, baseUrl} from './utils.ts';

test('assign issue to project and change column', async ({page}) => {
  const repoName = `e2e-issue-project-${Date.now()}`;
  const user = env.GITEA_TEST_E2E_USER;
  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  // Create a project via web form
  await page.goto(`/${user}/${repoName}/projects/new`);
  await page.locator('input[name="title"]').fill('Kanban Board');
  await page.getByRole('button', {name: 'Create Project'}).click();

  // Extract project ID from the listed project link
  const projectLink = page.locator('.milestone-list a', {hasText: 'Kanban Board'}).first();
  await expect(projectLink).toBeVisible();
  const href = await projectLink.getAttribute('href');
  const projectID = href!.split('/').pop();

  // Add columns with colors via POST (the web UI uses modals which are hard to drive)
  for (const [title, color] of [['Backlog', '#0075ca'], ['In Progress', '#e4e669'], ['Done', '#0e8a16']]) {
    await page.request.post(`${baseUrl()}/${user}/${repoName}/projects/${projectID}/columns/new`, {
      headers: apiHeaders(),
      form: {title, color},
    });
  }

  // Create an issue via API
  const issueResp = await page.request.post(`${baseUrl()}/api/v1/repos/${user}/${repoName}/issues`, {
    headers: apiHeaders(),
    data: {title: 'Test Issue for Column Picker'},
  });
  expect(issueResp.ok()).toBeTruthy();
  const issue = await issueResp.json();

  // Assign issue to the project via the sidebar
  await page.goto(`/${user}/${repoName}/issues/${issue.number}`);
  await page.locator('.sidebar-project-combo .ui.dropdown').click();
  await Promise.all([
    page.waitForResponse((resp) => resp.url().includes('/issues/projects') && resp.status() === 200),
    page.locator('.sidebar-project-combo .menu .item', {hasText: 'Kanban Board'}).click(),
  ]);
  await page.waitForLoadState('load');

  // Verify the project card appears with a column dropdown
  await expect(page.locator('.sidebar-project-card')).toBeVisible();
  await expect(page.locator('.sidebar-project-card-header', {hasText: 'Kanban Board'})).toBeVisible();
  await expect(page.locator('.sidebar-project-column-combo')).toBeVisible();

  // Verify column dropdown is present and click to open
  const columnCombo = page.locator('.sidebar-project-column-combo');
  await expect(columnCombo).toBeVisible({timeout: 10000});
  await columnCombo.locator('.ui.dropdown').click();

  // Verify color dots are present in the dropdown
  await expect(columnCombo.locator('.menu .sidebar-column-dot').first()).toBeVisible();

  // Select "In Progress" column
  await Promise.all([
    page.waitForResponse((resp) => resp.url().includes('/issues/projects/column') && resp.status() === 200),
    columnCombo.locator('.menu .item', {hasText: 'In Progress'}).click(),
  ]);
  await page.waitForLoadState('load');

  // Verify the column changed
  await expect(page.locator('.sidebar-project-column-combo .sidebar-project-column-text')).toContainText('In Progress');

  // Verify timeline shows the column move
  await expect(page.locator('.timeline-item', {hasText: 'moved this to In Progress'})).toBeVisible();

  // Verify the checked state in the dropdown
  await page.locator('.sidebar-project-column-combo .ui.dropdown').click();
  const checkedItem = page.locator('.sidebar-project-column-combo .menu .item.checked');
  await expect(checkedItem).toContainText('In Progress');

  await apiDeleteRepo(page.request, user, repoName);
});
