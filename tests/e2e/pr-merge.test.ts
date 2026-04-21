import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import type {APIRequestContext} from '@playwright/test';
import {apiCreateFile, apiCreatePR, apiCreateRepo, login, randomString} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

async function setupMergeablePR(request: APIRequestContext) {
  const repoName = `e2e-prmerge-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName});
  await apiCreateFile(request, owner, repoName, 'feature.txt', 'hello\n', {branch: 'main', newBranch: 'feat'});
  const prIndex = await apiCreatePR(request, owner, repoName, 'feat', 'main', 'add feature');
  return {repoName, prIndex};
}

test.describe('pr merge', () => {
  test('default merge', async ({page, request}) => {
    const [, {repoName, prIndex}] = await Promise.all([login(page), setupMergeablePR(request)]);
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

  test('squash merge', async ({page, request}) => {
    const [, {repoName, prIndex}] = await Promise.all([login(page), setupMergeablePR(request)]);
    await page.goto(`/${owner}/${repoName}/pulls/${prIndex}`);

    const mergeButton = page.locator('.merge-button');
    await mergeButton.locator('.dropdown').click();
    await mergeButton.locator('.menu .action-text', {hasText: 'Create squash commit'}).click();
    await expect(mergeButton.locator('.button-text')).toContainText('Create squash commit');

    await mergeButton.locator('button.ui.button').first().click();
    await page.getByRole('button', {name: 'Create squash commit', exact: true}).click();

    await expect(page.locator('.issue-state-label')).toContainText('Merged');

    // squash => main must have exactly 2 commits (initial README + squashed), and the tip must have a single parent (not a merge)
    const commitsResponse = await request.get(`/api/v1/repos/${owner}/${repoName}/commits?sha=main&limit=10`);
    const commits = await commitsResponse.json();
    expect(commits).toHaveLength(2);
    expect(commits[0].parents).toHaveLength(1);
  });
});
