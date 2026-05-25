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

  // posting reloaded the page with an empty box, so the status button now reads "Close Issue"
  await page.getByRole('button', {name: 'Close Issue'}).click();
  await expect(page.getByRole('button', {name: 'Reopen Issue'})).toBeVisible();
});
