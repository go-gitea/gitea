import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo, apiBaseUrl, apiHeaders} from './utils.ts';

test('projects + create an issue + create a pull request (PR via API)', async ({page}) => {
  test.setTimeout(30_000);

  const owner = env.GITEA_TEST_E2E_USER!;
  const repoName = `e2e-projects-${Date.now()}`;

  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  // Create first project
  await page.goto(`/${owner}/${repoName}/projects/new`);
  await page.getByPlaceholder('Title').fill('Test Project - No default column');
  await page.getByRole('button', {name: 'Create Project'}).click();
  await expect(page.locator('.project-list')).toContainText('Test Project - No default column');

  // Create project with default template
  await page.goto(`/${owner}/${repoName}/projects/new`);
  await page.getByRole('textbox', { name: 'Title' }).fill('Test Project - Default column available');
  await page.getByText('Select a project template to get started None Basic Kanban Bug Triage').click();
  await page.getByRole('option', { name: 'Basic Kanban' }).click();
  await page.getByRole('button', { name: 'Create Project' }).click();
  await expect(page.locator('.project-list')).toContainText('Test Project - Default column available');

    // Create an issue via UI and verify
  await page.goto(`/${owner}/${repoName}/issues/new`);
  await page.getByPlaceholder('Title').fill('E2E Test Issue - 1');
  await page.getByRole('button', {name: 'Create Issue'}).click();
  await expect(page.locator('.issue-title h1')).toContainText('E2E Test Issue - 1');
  await page.goto(`/${owner}/${repoName}/issues`);
  await expect(page.locator('#issue-list')).toContainText('E2E Test Issue - 1');

  // Case 1: Assign both projects to the issue above
  await page.getByRole('link', { name: 'E2E Test Issue - 1' }).click();
  await page.locator('a').filter({ hasText: 'Projects' }).nth(2).click();
  await page.getByRole('link', { name: 'Test Project - No default column' }).click();
  await page.getByRole('link', { name: 'Test Project - Default column available' }).click();
  await page.mouse.click(1, 1);
  await expect(page.locator('menu.visible.transition')).toBeHidden();
  await expect(page.getByText('test_repo added this to the Test Project - No default column')).toBeVisible();
  await expect(page.getByText('test_repo added this to the Test Project - Default column available')).toBeVisible();

  await page.goto(`/${owner}/${repoName}/projects`);
  await page.getByRole('link', { name: 'Test Project - No default column' }).click();
  await expect(page.getByText('Uncategorized')).toBeVisible();
  await expect(page.getByText('E2E Test Issue - 1')).toBeVisible();

  await page.goto(`/${owner}/${repoName}/projects`);
  await page.getByRole('link', { name: 'Test Project - Default column available' }).click();
  await expect(page.getByText('Backlog')).toBeVisible();
  await expect(page.getByText('E2E Test Issue - 1')).toBeVisible();

  // Add a 2nd issue to the same projects, expect sorting to be correct
  await page.goto(`/${owner}/${repoName}/issues/new`);
  await page.getByPlaceholder('Title').fill('E2E Test Issue - 2');
  await page.getByRole('button', {name: 'Create Issue'}).click();
  await expect(page.locator('.issue-title h1')).toContainText('E2E Test Issue - 2');
  await page.goto(`/${owner}/${repoName}/issues`);
  await expect(page.locator('#issue-list')).toContainText('E2E Test Issue - 2');

  await page.goto(`/${owner}/${repoName}/issues/2`);
  await page.locator('a').filter({ hasText: 'Projects' }).nth(2).click();
  await page.getByRole('link', { name: 'Test Project - Default column available' }).click();
  await page.mouse.click(1, 1);
  await expect(page.locator('menu.visible.transition')).toBeHidden();
  await expect(page.getByText('test_repo added this to the Test Project - Default column available')).toBeVisible();

  await page.goto(`/${owner}/${repoName}/projects`);
  await page.getByRole('link', { name: 'Test Project - Default column available' }).click();
  await expect(page.locator('.issue-card-title')).toContainText(['E2E Test Issue - 1', 'E2E Test Issue - 2']);

  // Remove issue from all projects
  await page.goto(`/${owner}/${repoName}/issues/1`);
  await page.locator('a').filter({ hasText: 'Projects' }).nth(2).click();
  await page.getByText('Clear projects').click();
  await page.mouse.click(1, 1);
  await expect(page.locator('menu.visible.transition')).toBeHidden();
  await expect(page.getByText('test_repo removed this from the Test Project - No default column')).toBeVisible();
  await expect(page.getByText('test_repo removed this from the Test Project - No default column')).toBeVisible();

  await page.goto(`/${owner}/${repoName}/projects`);
  await page.getByRole('link', { name: 'Test Project - No default column' }).click();
  await expect(page.getByText('Uncategorized')).toBeVisible();
  await expect(page.getByText('E2E Test Issue - 1')).toHaveCount(0);

  // Cleanup
  await apiDeleteRepo(page.request, owner, repoName);
});
