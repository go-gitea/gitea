import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateBranch, apiCreateFiles, apiCreateRepo, apiHeaders, randomString} from './utils.ts';

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

test('file tree, go to file and tag selector navigate nested files', async ({page, request}) => {
  const repoName = `e2e-file-nav-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const tagName = 'review/café';
  const guideUrlPath = 'docs/%231%20guide.txt';
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      await apiCreateFiles(request, owner, repoName, [
        {path: 'start.txt', content: 'Starting file content\n'},
        {path: 'docs/#1 guide.txt', content: 'Nested guide content\n'},
      ]);
      expect((await request.post(`/api/v1/repos/${owner}/${repoName}/tags`, {
        headers: apiHeaders(), data: {tag_name: tagName},
      })).status()).toBe(201);
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/src/branch/main/start.txt`);
  const tree = page.locator('#view-file-tree');
  await tree.getByRole('link', {name: 'docs', exact: true}).locator('.item-toggle svg').click();
  await tree.getByRole('link', {name: '#1 guide.txt', exact: true}).click();
  const guideContent = page.getByText('Nested guide content', {exact: true});
  await expect(guideContent).toBeVisible();
  await page.goBack();
  await expect(page.getByText('Starting file content', {exact: true})).toBeVisible();

  const search = page.getByRole('combobox', {name: 'Go to file'});
  await search.fill('docs/gu');
  await expect(page.getByRole('option')).toHaveText(['docs/#1 guide.txt']);
  await search.press('Enter');
  await page.waitForURL(`/${owner}/${repoName}/src/branch/main/${guideUrlPath}`);
  await expect(guideContent).toBeVisible();

  await page.getByText('main', {exact: true}).click();
  await expect(page.getByText('default', {exact: true})).toBeVisible();
  await page.getByRole('button', {name: 'Tags', exact: true}).click();
  await page.keyboard.insertText(tagName);
  const tagText = page.getByText(tagName, {exact: true});
  await expect(tagText).toBeVisible();
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await page.waitForURL(`/${owner}/${repoName}/src/tag/review/caf%C3%A9/${guideUrlPath}`);
  await expect(guideContent).toBeVisible();
  await expect(tagText).toBeVisible();
});
