import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateRepo, apiCreateIssue, apiDeleteRepo} from './utils.ts';

test('toggle issue reactions', async ({page, request}) => {
  const repoName = 'e2e-reactions';
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  await Promise.all([
    apiCreateIssue(request, owner, repoName, {title: 'Reaction test'}),
    login(page),
  ]);
  try {
    await page.goto(`/${owner}/${repoName}/issues/1`);

    const issueComment = page.locator('.timeline-item.comment.first');

    await issueComment.getByLabel('Add reaction').click();
    await issueComment.getByLabel('Add reaction').getByLabel('+1').click();

    const reactions = issueComment.getByRole('group', {name: 'Reactions'});
    await expect(reactions.getByRole('button', {name: /^\+1:/})).toContainText('1');

    await reactions.getByRole('button', {name: /^\+1:/}).click();
    await expect(reactions.getByRole('button', {name: /^\+1:/})).toHaveCount(0);
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});
