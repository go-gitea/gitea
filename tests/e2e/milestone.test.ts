import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo} from './utils.ts';

test('create a milestone', async ({page}) => {
  const repoName = `e2e-milestone-${Date.now()}`;
  await login(page);
  await apiCreateRepo(page.request, {name: repoName});
  await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/milestones/new`);
  await page.getByPlaceholder('Title').fill('Test Milestone');
  await page.getByRole('button', {name: 'Create Milestone'}).click();
  await expect(page.locator('.milestone-list')).toContainText('Test Milestone');
  await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
});
