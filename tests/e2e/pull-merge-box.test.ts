import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateFile, apiCreatePR, apiCreateRepo, assertNoJsError, login, randomString} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('merge box switches style and renders the merge form', async ({page, request}) => {
  const repo = `e2e-merge-box-${randomString(8)}`;
  const createPR = (async () => {
    await apiCreateRepo(request, {name: repo});
    await apiCreateFile(request, owner, repo, 'feat.txt', 'feature\n', {branch: 'main', newBranch: 'feat'});
    return apiCreatePR(request, owner, repo, 'feat', 'main', 'merge box test');
  })();
  const [index] = await Promise.all([createPR, login(page)]);
  await page.goto(`/${owner}/${repo}/pulls/${index}`, {waitUntil: 'commit'});

  const mergeBox = page.locator('.pull-merge-box');
  const mergeButton = mergeBox.locator('.merge-button');
  await expect(mergeButton).toContainText('Create merge commit');

  await mergeButton.locator('.ui.dropdown').click();
  await mergeButton.locator('.menu').getByText('Create squash commit', {exact: true}).click();
  await expect(mergeButton).toContainText('Create squash commit');

  await mergeButton.locator('button.ui.button').first().click();
  const form = mergeBox.locator('form');
  await expect(form.locator('input[name="merge_title_field"]')).toHaveValue(new RegExp(`#${index}`));
  await expect(mergeBox.locator('#delete-branch-after-merge')).toBeAttached();

  await assertNoJsError(page);
});
