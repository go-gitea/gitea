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
  await expect(page.getByText('src/main.go', {exact: true})).toBeVisible();
});

test('file tree, history and go to file navigate between nested files', async ({page, request}) => {
  const repoName = `e2e-file-nav-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      await apiCreateFiles(request, owner, repoName, [
        {path: 'start.txt', content: 'Starting file content\n'},
        {path: 'docs/guide.txt', content: 'Nested guide content\n'},
      ]);
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/src/branch/main/start.txt`);
  const tree = page.locator('#view-file-tree');
  await tree.getByRole('link', {name: 'docs', exact: true}).locator('.item-toggle svg').click();
  await tree.getByRole('link', {name: 'guide.txt', exact: true}).click();
  await expect(page.getByText('Nested guide content', {exact: true})).toBeVisible();
  await page.goBack();
  await expect(page.getByText('Starting file content', {exact: true})).toBeVisible();

  const search = page.getByRole('combobox', {name: 'Go to file'});
  await search.fill('docs/gu');
  await expect(page.getByRole('option')).toHaveText(['docs/guide.txt']);
  await search.press('Enter');
  await expect(page.getByText('Nested guide content', {exact: true})).toBeVisible();
});
