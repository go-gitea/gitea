import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateBranchProtection, apiCreateFiles, apiCreatePR, apiCreateRepo, apiHeaders, assertNoJsError, baseUrl, login, randomString} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('merge queue merges a pull request after its queue checks pass', async ({page, request}) => {
  const repo = `e2e-merge-queue-${randomString(8)}`;
  const api = `${baseUrl()}/api/v1/repos/${owner}/${repo}`;
  const setStatus = async (sha: string) => {
    expect((await request.post(`${api}/statuses/${sha}`, {headers: apiHeaders(), data: {state: 'success', context: 'ci'}})).ok()).toBe(true);
  };
  const createPR = (async () => {
    await apiCreateRepo(request, {name: repo});
    await apiCreateFiles(request, owner, repo, [{path: 'feat.txt', content: 'feature\n'}], {branch: 'main', newBranch: 'feat'});
    const index = await apiCreatePR(request, owner, repo, 'feat', 'main', 'merge queue test');
    await apiCreateBranchProtection(request, owner, repo, 'main', {enable_status_check: true, status_check_contexts: ['ci'], require_merge_queue: true});
    let headSHA = '';
    await expect(async () => {
      const pr = await (await request.get(`${api}/pulls/${index}`, {headers: apiHeaders()})).json();
      expect(pr.mergeable).toBe(true);
      headSHA = pr.head.sha;
    }).toPass();
    await setStatus(headSHA);
    return index;
  })();
  const [index] = await Promise.all([createPR, login(page)]);
  await page.goto(`/${owner}/${repo}/pulls/${index}`, {waitUntil: 'commit'});

  // expand the merge form, then submit it
  await page.getByRole('button', {name: 'Add to merge queue'}).click();
  await page.locator('form.form-fetch-action').getByRole('button', {name: 'Add to merge queue'}).click();
  await expect(page.getByText('This pull request is at position 1')).toBeVisible();

  await page.getByRole('link', {name: 'merge queue of main'}).click();
  await expect(page.getByRole('link', {name: 'merge queue test'})).toBeVisible();

  let groupSHA = '';
  await expect(async () => {
    const queue = await (await request.get(`${api}/merge-queue/main`, {headers: apiHeaders()})).json();
    groupSHA = queue[0].merge_group_head_sha;
    expect(groupSHA).not.toBe('');
  }).toPass();
  await setStatus(groupSHA);

  await expect(async () => {
    const pr = await (await request.get(`${api}/pulls/${index}`, {headers: apiHeaders()})).json();
    expect(pr.merged).toBe(true);
    expect(pr.merge_commit_sha).toBe(groupSHA);
  }).toPass();
  await page.goto(`/${owner}/${repo}/pulls/${index}`);
  await expect(page.getByText('Merged', {exact: true})).toBeVisible();
  await assertNoJsError(page);
});
