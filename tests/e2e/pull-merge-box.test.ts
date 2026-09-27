import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  apiCreateBranchProtection, apiCreateCommitStatus, apiCreateFiles, apiCreatePR, apiCreateRepo, apiGetPR, assertNoJsError, login,
  randomString,
} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('merge box toggles auto merge and merges bypassing branch protection', async ({page, request}) => {
  const repo = `e2e-merge-box-${randomString(8)}`;
  await Promise.all([apiCreateRepo(request, {name: repo}), login(page)]);
  await apiCreateFiles(request, owner, repo, [{path: 'feat.txt', content: 'feature\n'}], {branch: 'main', newBranch: 'feat'});
  await apiCreateBranchProtection(request, owner, repo, {rule_name: 'main', required_approvals: 1});
  const index = await apiCreatePR(request, owner, repo, 'feat', 'main', 'merge box test');
  let headSha = '';
  await expect.poll(async () => {
    const pull = await apiGetPR(request, owner, repo, index);
    headSha = pull.head.sha;
    return pull.mergeable;
  }).toBe(true);
  await apiCreateCommitStatus(request, owner, repo, headSha, {context: 'ci/test', state: 'failure'});
  await page.goto(`/${owner}/${repo}/pulls/${index}`, {waitUntil: 'commit'});

  await expect(page.getByRole('heading', {name: 'Review required', exact: true})).toBeVisible();
  await expect(page.getByText('All checks have failed')).toBeVisible();
  await page.getByRole('button', {name: 'Enable auto-merge (merge commit)'}).click();
  await page.getByRole('button', {name: 'Confirm auto-merge (merge commit)'}).click();
  await expect(page.getByText('enabled auto-merge (merge commit)')).toBeVisible();
  await page.getByRole('button', {name: 'Disable auto-merge'}).click();

  await page.getByLabel('Merge without waiting for requirements to be met (bypass rules)').check();
  await page.getByRole('button', {name: 'Bypass rules and merge (merge commit)'}).click();
  await page.getByRole('button', {name: 'Confirm bypass rules and merge (merge commit)'}).click();

  await expect(page.getByText(/successfully merged/i)).toBeVisible();
  await assertNoJsError(page);
});
