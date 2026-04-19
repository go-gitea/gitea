import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateIssue, apiDeleteRepo, createProjectColumn, randomString, timeoutFactor} from './utils.ts';

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
  const projectID = href!.split('/').pop();

  // columns created via POST because the web UI uses modals that are hard to drive
  await Promise.all(['Backlog', 'In Progress', 'Done'].map((title) =>
    createProjectColumn(page.request, user, repoName, projectID!, title),
  ));

  await apiCreateIssue(page.request, user, repoName, {title: 'Column picker test'});

  // Same ceiling as tests/e2e/events.test.ts; Playwright defaults are 5000*factor (see playwright.config.ts).
  const slowTimeout = 15_000 * timeoutFactor;

  await page.goto(`/${user}/${repoName}/issues/1`);
  await page.locator('.sidebar-project-combo .ui.dropdown').click();
  await Promise.all([
    page.waitForResponse(
      (resp) => resp.url().includes('/issues/projects') && resp.status() === 200,
      {timeout: slowTimeout},
    ),
    page.locator('.sidebar-project-combo .menu .item', {hasText: 'Kanban Board'}).click(),
  ]);

  const columnCombo = page.locator('.sidebar-project-column-combo');
  await expect(columnCombo).toBeVisible({timeout: slowTimeout});
  await columnCombo.locator('.ui.dropdown').click();
  await columnCombo.locator('.menu').waitFor({state: 'visible', timeout: slowTimeout});

  const inProgressItem = columnCombo.locator('a.item', {hasText: 'In Progress'});
  await expect(inProgressItem).toBeVisible({timeout: slowTimeout});
  await inProgressItem.scrollIntoViewIfNeeded();

  await Promise.all([
    page.waitForResponse(
      (resp) =>
        resp.request().method() === 'POST' &&
        resp.url().includes('/issues/projects/column') &&
        resp.ok(),
      {timeout: slowTimeout},
    ),
    inProgressItem.click(),
  ]);

  await expect(columnCombo.getByTestId('sidebar-project-column-text')).toContainText('In Progress', {
    timeout: slowTimeout,
  });
  await expect(page.locator('.timeline-item', {hasText: 'moved this to In Progress'})).toBeVisible({
    timeout: slowTimeout,
  });

  await apiDeleteRepo(page.request, user, repoName);
});
