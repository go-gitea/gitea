import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  apiCreateBranchProtection, apiCreateFiles, apiCreatePR, apiCreateRepo, apiHeaders, baseUrl, login, randomString,
} from './utils.ts';

test('the merge queue offers admin bypass and shows missing checks and automatic removal', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `e2e-merge-queue-${randomString(8)}`;
  const api = `${baseUrl()}/api/v1/repos/${owner}/${repo}`;
  const setStatus = async (sha: string, state: 'pending' | 'success' | 'failure') => {
    await expect(await request.post(`${api}/statuses/${sha}`, {headers: apiHeaders(), data: {state, context: 'ci'}})).toBeOK();
  };
  await Promise.all([apiCreateRepo(request, {name: repo}), login(page)]);
  await Promise.all([
    apiCreateFiles(request, owner, repo, [{path: 'feat.txt', content: 'feature\n'}], {branch: 'main', newBranch: 'feat'}),
    apiCreateBranchProtection(request, owner, repo, 'main', {enable_status_check: true, status_check_contexts: ['ci'], require_merge_queue: true}),
  ]);
  const index = await apiCreatePR(request, owner, repo, 'feat', 'main', 'merge queue test');
  const pullLink = `/${owner}/${repo}/pulls/${index}`;
  let headSHA = '';
  await expect(async () => {
    const pr = await (await request.get(`${api}/pulls/${index}`, {headers: apiHeaders()})).json();
    expect(pr.mergeable).toBe(true);
    headSHA = pr.head.sha;
  }).toPass();
  await setStatus(headSHA, 'success');
  await page.goto(pullLink, {waitUntil: 'commit'});

  const bypass = page.getByLabel('Merge without waiting for requirements to be met (bypass rules)');
  await bypass.check();
  await expect(page.getByRole('button', {name: 'Bypass rules and merge (merge commit)'})).toBeVisible();
  await bypass.uncheck();
  await expect(page.getByRole('button', {name: 'Select merge style'})).toBeHidden();
  await page.getByRole('button', {name: 'Merge when ready', exact: true}).click();
  await page.getByRole('button', {name: 'Confirm merge when ready', exact: true}).click();

  const getQueue = async () => (await request.get(`${api}/merge-queue/main`, {headers: apiHeaders()})).json();
  await expect.poll(async () => (await getQueue())[0]?.state).toBe('awaiting_checks');
  await page.getByText('This pull request is next up in the merge queue.').getByRole('link', {name: 'merge queue'}).click();
  await expect(page.getByText('Waiting for ci')).toBeVisible();
  await setStatus((await getQueue())[0].merge_group_head_sha, 'failure');
  await expect.poll(getQueue).toEqual([]);

  await page.goto(pullLink, {waitUntil: 'commit'});
  const removal = page.locator('.timeline-item.event').filter({has: page.getByText('Removed from the merge queue due to failed status checks')});
  await expect(removal).toBeVisible();
  await expect(removal.getByRole('link')).toHaveCount(1);
});
