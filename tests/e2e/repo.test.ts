import {env} from 'node:process';
import {test} from '@playwright/test';
import {login, randomString} from './utils.ts';

test('create a repository', async ({page}) => {
  const repoName = `e2e-repo-${randomString(8)}`;
  await login(page);
  await page.goto('/repo/create');
  await page.locator('input[name="repo_name"]').fill(repoName);
  await page.getByRole('button', {name: 'Create Repository'}).click();
  await page.waitForURL(new RegExp(`/${env.GITEA_TEST_E2E_USER}/${repoName}$`));
});
