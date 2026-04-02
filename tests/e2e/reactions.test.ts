import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateRepo, apiCreateIssue, apiDeleteRepo, randomString} from './utils.ts';

test('toggle issue reactions', async ({page, request}) => {
  const repoName = `e2e-reactions-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  await Promise.all([
    apiCreateIssue(request, owner, repoName, {title: 'Reaction test'}),
    login(page),
  ]);
  try {
    await page.goto(`/${owner}/${repoName}/issues/1`);

    const issueComment = page.locator('.timeline-item.comment.first');

    await issueComment.locator('.select-reaction').click();
    await issueComment.locator('.select-reaction .item[data-reaction-content="+1"]').click();
    await expect(issueComment.locator('.bottom-reactions a[role="button"][data-reaction-content="+1"] .reaction-count')).toHaveText('1');

    await issueComment.locator('.bottom-reactions a[role="button"][data-reaction-content="+1"]').click();
    await expect(issueComment.locator('.bottom-reactions a[role="button"][data-reaction-content="+1"]')).toHaveCount(0);
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});
