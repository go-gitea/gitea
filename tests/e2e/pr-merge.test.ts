import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateFile, apiCreatePR, apiCreateRepo, login, randomString} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('default merge', async ({page, request}) => {
  const repoName = `e2e-prmerge-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName});
  await apiCreateFile(request, owner, repoName, 'feature.txt', 'hello\n', {branch: 'main', newBranch: 'feat'});
  const [, prIndex] = await Promise.all([
    login(page),
    apiCreatePR(request, owner, repoName, 'feat', 'main', 'add feature'),
  ]);
  await page.goto(`/${owner}/${repoName}/pulls/${prIndex}`);

  await page.locator('.merge-button button.ui.button').first().click();
  // default repo config has delete-branch-after-merge OFF
  await expect(page.getByLabel(/^Delete Branch/)).not.toBeChecked();
  await page.getByRole('button', {name: 'Create merge commit', exact: true}).click();

  await expect(page.locator('.issue-state-label')).toContainText('Merged');
  await expect(page.getByText('Pull request successfully merged and closed')).toBeVisible();
  await expect(page.locator('.merge-section .delete-branch-after-merge')).toBeVisible();
  await expect(page.locator('.merge-button')).toBeHidden();

  const branchResponse = await request.get(`/api/v1/repos/${owner}/${repoName}/branches/feat`);
  expect(branchResponse.status()).toBe(200);
});
