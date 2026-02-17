import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, deleteRepo} from './utils.ts';

test('create a repository', async ({page}) => {
  const repoName = `e2e-repo-${Date.now()}`;
  await login(page);
  await page.goto('/repo/create');
  await page.locator('input[name="repo_name"]').fill(repoName);
  await page.getByRole('button', {name: 'Create Repository'}).click();
  await expect(page).toHaveURL(new RegExp(`/${env.E2E_USER}/${repoName}$`));
  await deleteRepo(page, env.E2E_USER, repoName);
});
