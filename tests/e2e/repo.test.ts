import {env} from 'node:process';
import {test} from '@playwright/test';
import {login, apiDeleteRepo} from './utils.ts';

test('create a repository', async ({page}) => {
  const repoName = `e2e-repo-${Date.now()}`;
  await login(page);
  await page.goto('/repo/create');
  await page.locator('input[name="repo_name"]').fill(repoName);
  await page.getByRole('button', {name: 'Create Repository'}).click();
  await page.waitForURL(new RegExp(`/${env.GITEA_TEST_E2E_USER}/${repoName}$`));
  await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
});
