import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateRepo, apiCreateIssue, randomString} from './utils.ts';

test('toggle issue reactions', async ({page, request}) => {
  const repoName = `e2e-reactions-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName, autoInit: false});
  await Promise.all([
    apiCreateIssue(request, {owner, repo: repoName, title: 'Reaction test'}),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/issues/1`);

  const reactions = page.locator('.timeline-item.comment.issue-content-comment').getByRole('group', {name: 'Reactions'});
  const reactionPicker = reactions.getByRole('button', {name: 'Reactions', exact: true});

  await reactionPicker.click();
  await page.getByRole('menuitem', {name: '+1', exact: true}).click();
  await expect(reactions.getByRole('button', {name: /^\+1:/})).toContainText('1');

  await reactions.getByRole('button', {name: /^\+1:/}).click();
  await expect(reactions.getByRole('button', {name: /^\+1:/})).toHaveCount(0);

  await reactionPicker.press('Enter');
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('Enter');
  await expect(reactions.getByRole('button', {name: /^-1:/})).toContainText('1');
  await expect(reactionPicker).toBeFocused();
});
