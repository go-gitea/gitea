import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateBranch, apiCreateFiles, apiCreateRepo, randomString} from './utils.ts';

test('create a repository', async ({page}) => {
  const repoName = `e2e-repo-${randomString(8)}`;
  await login(page);
  await page.goto('/repo/create');
  await page.locator('input[name="repo_name"]').fill(repoName);
  await page.getByRole('button', {name: 'Create Repository'}).click();
  await page.waitForURL(new RegExp(`/${env.GITEA_TEST_E2E_USER}/${repoName}$`));
});

test('delete a branch via its modal', async ({page, request}) => {
  const repoName = `e2e-branch-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName});
      await apiCreateBranch(request, owner, repoName, 'feature');
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/branches`);
  await page.getByRole('button', {name: 'Delete Branch "feature"'}).click();
  const modal = page.getByRole('dialog');
  await expect(modal).toContainText('feature');
  await modal.getByRole('button', {name: 'Yes'}).click();
  await expect(page.locator('.flash-message')).toContainText('Branch "feature" has been deleted.');
});

test('search code', async ({page, request}) => {
  const repoName = `e2e-search-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName});
      await apiCreateFiles(request, owner, repoName, [{path: 'src/main.go', content: 'package main\n\nfunc alphaTerm() {}\n\nvar betaTerm = 1\n'}]);
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/search`);
  const input = page.getByPlaceholder('Search code…');
  await input.fill('alphaterm "betaTerm = 1"');
  await input.press('Enter');
  await page.waitForURL(/[?&]q=alphaterm/);
  await expect(page.locator('.repo-search-result')).toContainText('src/main.go');
  await input.fill('"unterminated');
  await input.press('Enter');
  await page.waitForURL(/[?&]q=%22unterminated/);
  await expect(page.locator('.ui.error.message')).toContainText('Invalid search query');
});
