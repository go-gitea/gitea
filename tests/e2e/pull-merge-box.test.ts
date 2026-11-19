import {env} from 'node:process';
import {test, expect, type APIRequestContext} from '@playwright/test';
import {
  apiCreateBranchProtection, apiCreateFiles, apiCreatePR, apiCreateRepo, apiHeaders, assertNoJsError, login, randomString,
} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

async function apiCreateMergeablePR(request: APIRequestContext, prepareRepo?: (repo: string) => Promise<unknown>, base = 'main') {
  const repo = `e2e-merge-box-${randomString(8)}`;
  await apiCreateRepo(request, {name: repo});
  await Promise.all([
    apiCreateFiles(request, owner, repo, [{path: 'feat.txt', content: 'feature\n'}], {branch: 'main', newBranch: 'feat'}),
    prepareRepo?.(repo),
  ]);
  return `/${owner}/${repo}/pulls/${await apiCreatePR(request, owner, repo, 'feat', base, 'merge box test')}`;
}

test('auto-merge can be canceled before a squash bypass merge', async ({page, request}) => {
  const [prLink] = await Promise.all([
    apiCreateMergeablePR(request, (repo) => apiCreateBranchProtection(request, owner, repo, 'main', {required_approvals: 1})),
    login(page),
  ]);
  await page.goto(prLink, {waitUntil: 'commit'});

  await page.getByRole('button', {name: 'Enable auto-merge (merge commit)'}).click();
  await page.getByRole('button', {name: 'Confirm auto-merge (merge commit)'}).click();
  await expect(page.getByText('enabled auto-merge (merge commit)')).toBeVisible();
  await page.waitForLoadState();
  await page.getByRole('button', {name: 'View command line instructions.'}).press('Enter');
  const commandDialog = page.getByRole('dialog', {name: 'Command line instructions'});
  await expect(commandDialog.getByRole('heading', {name: 'Checkout', exact: true})).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(commandDialog).toBeHidden();
  await page.getByRole('button', {name: 'Disable auto-merge'}).click();
  await expect(page.getByText('disabled auto-merge')).toBeVisible();

  await page.getByRole('button', {name: 'Select merge style'}).click();
  await page.getByRole('menuitemradio', {name: /^Create squash commit/}).click();
  await page.getByLabel('Merge without waiting for requirements to be met (bypass rules)').check();
  await page.getByRole('button', {name: 'Bypass rules and merge (squash)'}).click();
  await page.getByRole('button', {name: 'Confirm bypass rules and merge (squash)'}).click();

  await expect(page.getByText('Merged', {exact: true})).toBeVisible();
  await assertNoJsError(page);
});

test('outdated branch can be rebased from the update style menu', async ({page, request}) => {
  const [prLink] = await Promise.all([
    apiCreateMergeablePR(request, (repo) => apiCreateFiles(request, owner, repo, [{path: 'base.txt', content: 'base\n'}], {branch: 'main', newBranch: 'base'}), 'base'),
    login(page),
  ]);
  await page.goto(prLink);
  await page.getByRole('button', {name: 'Update branch options'}).click();
  await page.getByRole('menuitemradio', {name: /^Update with rebase/}).click();
  const [updateRequest] = await Promise.all([
    page.waitForRequest((req) => req.method() === 'POST' && new URL(req.url()).pathname === `${prLink}/update`),
    page.getByRole('button', {name: 'Rebase branch'}).click(),
  ]);
  expect(new URL(updateRequest.url()).searchParams.get('style')).toBe('rebase');
  await expect(page.getByText('Branch update was successful')).toBeVisible();
  await assertNoJsError(page);
});

test('branch deleted by a merge can be restored from the timeline', async ({page, request}) => {
  const [prLink] = await Promise.all([
    apiCreateMergeablePR(request),
    login(page),
  ]);
  expect((await request.post(`/api/v1/repos${prLink}/merge`, {headers: apiHeaders(), data: {Do: 'merge', delete_branch_after_merge: true}})).ok()).toBe(true);
  await page.goto(prLink);
  await page.getByRole('button', {name: 'Restore branch'}).click();
  await expect(page.getByText('Branch "feat" has been restored.')).toBeVisible();
  await assertNoJsError(page);
});
