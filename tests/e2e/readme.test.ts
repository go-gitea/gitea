import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateRepo, apiDeleteRepo} from './utils.ts';

test('README renders on repository page', async ({page}) => {
  const repoName = `e2e-readme-${Date.now()}`;
  await apiCreateRepo(page.request, {name: repoName});
  await page.goto(`/${env.E2E_USER}/${repoName}`);
  await expect(page.locator('#readme')).toContainText(repoName);
  await apiDeleteRepo(page.request, env.E2E_USER, repoName);
});
