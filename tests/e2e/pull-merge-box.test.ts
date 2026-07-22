import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateFile, apiCreatePR, apiCreateRepo, assertNoJsError, login, randomString} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('merge box merges a pull request', async ({page, request}) => {
  const repo = `e2e-merge-box-${randomString(8)}`;
  const createPR = (async () => {
    await apiCreateRepo(request, {name: repo});
    await apiCreateFile(request, owner, repo, 'feat.txt', 'feature\n', {branch: 'main', newBranch: 'feat'});
    return apiCreatePR(request, owner, repo, 'feat', 'main', 'merge box test');
  })();
  const [index] = await Promise.all([createPR, login(page)]);
  await page.goto(`/${owner}/${repo}/pulls/${index}`, {waitUntil: 'commit'});

  // expand the merge form, then submit the merge
  await page.getByRole('button', {name: 'Create merge commit'}).click();
  await page.locator('form.form-fetch-action').getByRole('button', {name: 'Create merge commit'}).click();

  await expect(page.getByText(/successfully merged/i)).toBeVisible();
  await assertNoJsError(page);
});
