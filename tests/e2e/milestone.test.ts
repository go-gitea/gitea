import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, randomString} from './utils.ts';

test('create a milestone', async ({page}) => {
  const repoName = `e2e-milestone-${randomString(8)}`;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/milestones/new`);
  await page.getByPlaceholder('Title').fill('Test Milestone');
  await page.getByRole('button', {name: 'Create Milestone'}).click();
  await expect(page.locator('.milestone-list')).toContainText('Test Milestone');
});
