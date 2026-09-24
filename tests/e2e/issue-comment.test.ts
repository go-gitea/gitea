import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateIssue, randomString} from './utils.ts';

test('comment on and close an issue', async ({page, request}) => {
  const repoName = `e2e-issue-comment-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName, autoInit: false});
  await Promise.all([
    apiCreateIssue(request, {owner, repo: repoName, title: 'Comment test'}),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/issues/1`);

  const body = `e2e-comment-${randomString(8)}`;
  await page.getByPlaceholder('Leave a comment').fill(body);
  // exact match: the status button reads "Close with Comment" while the box has content, which substring-matches "Comment"
  await page.getByRole('button', {name: 'Comment', exact: true}).click();
  await expect(page.locator('.comment-body').filter({hasText: body})).toBeVisible();

  // wait for the form to re-initialize (the empty box disables the comment button); a close click
  // before that does a native submit which lands on a raw JSON page instead of reloading the issue
  await expect(page.getByRole('button', {name: 'Comment', exact: true})).toBeDisabled();
  await page.getByRole('button', {name: 'Close Issue'}).click();
  await expect(page.getByRole('button', {name: 'Reopen Issue'})).toBeVisible();
});

test('unsaved issue description prompts before leaving', async ({page, request}) => {
  const repoName = `e2e-are-you-sure-${randomString(8)}`;
  await Promise.all([apiCreateRepo(request, {name: repoName, autoInit: false}), login(page)]);
  await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/new`);
  await page.getByPlaceholder('Leave a comment').press('a');
  const dialogPromise = page.waitForEvent('dialog');
  page.once('dialog', (dialog) => dialog.dismiss());
  await page.getByRole('link', {name: 'Dashboard'}).click();
  expect((await dialogPromise).type()).toBe('beforeunload');
  await expect(page).toHaveURL(/\/issues\/new$/);
});
