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

test('comment on a commit diff line', async ({page, request}) => {
  const repoName = `e2e-commit-comment-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    apiCreateRepo(request, {name: repoName, autoInit: false}),
    login(page),
  ]);
  const sha = await apiCreateFiles(request, owner, repoName, [{path: 'added.txt', content: 'added line\n'}]);
  await page.goto(`/${owner}/${repoName}/commit/${sha}`);
  await page.getByRole('button', {name: 'Add line comment', exact: true}).click();
  await page.getByRole('textbox', {name: 'Leave a comment', exact: true}).fill('commit comment');
  await page.getByRole('button', {name: 'Add single comment', exact: true}).click();
  await page.getByRole('button', {name: 'Reply', exact: true}).click();
  await page.getByRole('textbox', {name: 'Leave a comment', exact: true}).fill('commit reply');
  await page.getByRole('button', {name: 'Reply', exact: true}).click();
  const reply = page.getByRole('paragraph').filter({hasText: 'commit reply'});
  await expect(reply).toBeVisible();

  const replyMenu = page.getByRole('menu').filter({has: page.getByRole('menuitem', {name: 'Delete', exact: true, includeHidden: true})}).last();
  await replyMenu.click();
  await replyMenu.getByRole('menuitem', {name: 'Delete', exact: true}).click();
  await page.getByRole('button', {name: 'Confirm', exact: true}).click();
  await expect(reply).toHaveCount(0);
  await expect(page.getByRole('paragraph').filter({hasText: 'commit comment'})).toBeVisible();
});
