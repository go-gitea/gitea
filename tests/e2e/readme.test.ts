import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, createRepo, deleteRepo} from './utils.ts';

test('README renders on repository page', async ({page}) => {
  const repoName = `e2e-readme-${Date.now()}`;
  await login(page);
  await createRepo(page, repoName);
  await page.goto(`/${env.E2E_USER}/${repoName}`);
  await expect(page.locator('#readme')).toContainText(repoName);
  await deleteRepo(page, env.E2E_USER, repoName);
});
