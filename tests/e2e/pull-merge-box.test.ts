import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  apiCreateBranchProtection, apiCreateFiles, apiCreatePR, apiCreateRepo, apiHeaders, assertNoJsError, login, randomString,
} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('merge box merges a pull request', async ({page, request}) => {
  const repo = `e2e-merge-box-${randomString(8)}`;
  const createPR = (async () => {
    await apiCreateRepo(request, {name: repo});
    await Promise.all([
      apiCreateFiles(request, owner, repo, [{path: 'feat.txt', content: 'feature\n'}], {branch: 'main', newBranch: 'feat'}),
      apiCreateBranchProtection(request, owner, repo, {rule_name: 'main', required_approvals: 1}),
    ]);
    const index = await apiCreatePR(request, owner, repo, 'feat', 'main', 'merge box test');
    await expect.poll(async () => (await (await request.get(`/api/v1/repos/${owner}/${repo}/pulls/${index}`, {headers: apiHeaders()})).json()).mergeable).toBe(true);
    return index;
  })();
  const [index] = await Promise.all([createPR, login(page)]);
  await page.goto(`/${owner}/${repo}/pulls/${index}`, {waitUntil: 'commit'});

  await page.getByRole('button', {name: 'Enable auto-merge (merge commit)'}).click();
  await page.getByRole('button', {name: 'Confirm auto-merge (merge commit)'}).click();
  await expect(page.getByText('enabled auto-merge (merge commit)')).toBeVisible();
  await page.waitForLoadState();
  await page.getByRole('button', {name: 'Disable auto-merge'}).click();

  await page.getByRole('button', {name: 'Select merge method'}).click();
  await page.getByRole('menuitemradio', {name: /^Create squash commit/}).click();
  await page.getByLabel('Merge without waiting for requirements to be met (bypass rules)').check();
  await page.getByRole('button', {name: 'Bypass rules and merge (squash)'}).click();
  await page.getByRole('button', {name: 'Confirm bypass rules and merge (squash)'}).click();

  await expect(page.getByText(/successfully merged/i)).toBeVisible();
  await assertNoJsError(page);
});
