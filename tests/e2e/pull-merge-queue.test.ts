import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  apiCreateBranchProtection, apiCreateFiles, apiCreatePR, apiCreateRepo, apiHeaders, baseUrl, login, randomString,
} from './utils.ts';

test('the admin can bypass the queue, see pending checks and see why a pull request was removed', async ({page, request}) => {
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
  await setStatus(headSHA, 'pending');
  await page.goto(pullLink, {waitUntil: 'commit'});

  const bypass = page.getByLabel('Merge without waiting for requirements to be met (bypass rules)');
  await page.getByRole('button', {name: 'Merge when ready', exact: true}).click();
  await expect(page.locator('[name="merge_when_checks_succeed"]')).toHaveValue('true');
  await expect(page.locator('[name="force_merge"]')).toHaveValue('false');

  await setStatus(headSHA, 'success');
  await page.reload({waitUntil: 'commit'});
  await expect(page.getByRole('button', {name: 'Select merge style'})).toBeHidden();
  await bypass.check();
  await page.getByRole('button', {name: 'Bypass rules and merge (merge commit)'}).click();
  await expect(page.locator('[name="force_merge"]')).toHaveValue('true');
  await page.getByRole('button', {name: 'Cancel'}).click();
  await bypass.uncheck();
  await page.getByRole('button', {name: 'Merge when ready', exact: true}).click();
  await page.getByRole('button', {name: 'Confirm merge when ready', exact: true}).click();

  const getQueue = async () => (await request.get(`${api}/merge-queue/main`, {headers: apiHeaders()})).json();
  let groupSHA = '';
  await expect(async () => {
    const [entry] = await getQueue();
    expect(entry).toMatchObject({state: 'awaiting_checks'});
    groupSHA = entry.merge_group_head_sha;
  }).toPass();
  await page.getByText('This pull request is next up in the merge queue.').getByRole('link', {name: 'merge queue'}).click();
  await expect(page.getByText('Waiting for ci')).toBeVisible();
  await setStatus(groupSHA, 'failure');
  await expect.poll(getQueue).toEqual([]);

  await page.goto(pullLink, {waitUntil: 'commit'});
  const removal = page.locator('.timeline-item.event').filter({hasText: 'Removed from the merge queue due to failed status checks'});
  await expect(removal).toBeVisible();
  await expect(removal).not.toContainText(owner);
});
