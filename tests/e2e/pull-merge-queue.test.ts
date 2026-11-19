import {env} from 'node:process';
import {test, expect, type APIRequestContext, type Page} from '@playwright/test';
import {
  apiCreateBranchProtection, apiCreateFiles, apiCreatePR, apiCreateRepo, apiHeaders, baseUrl, login, randomString,
} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

async function createQueueRequiredPR(page: Page, request: APIRequestContext, headState: 'pending' | 'success') {
  const repo = `e2e-merge-queue-${randomString(8)}`;
  const api = `${baseUrl()}/api/v1/repos/${owner}/${repo}`;
  const setStatus = async (sha: string, state: 'pending' | 'success' | 'failure') => {
    await expect(await request.post(`${api}/statuses/${sha}`, {headers: apiHeaders(), data: {state, context: 'ci'}})).toBeOK();
  };
  await Promise.all([apiCreateRepo(request, {name: repo}), login(page)]);
  const [index] = await Promise.all([
    (async () => {
      await apiCreateFiles(request, owner, repo, [{path: 'feat.txt', content: 'feature\n'}], {branch: 'main', newBranch: 'feat'});
      return apiCreatePR(request, owner, repo, 'feat', 'main', 'merge queue test');
    })(),
    apiCreateBranchProtection(request, owner, repo, 'main', {enable_status_check: true, status_check_contexts: ['ci'], require_merge_queue: true}),
  ]);
  const getPull = async () => (await request.get(`${api}/pulls/${index}`, {headers: apiHeaders()})).json();
  const getQueue = async () => (await request.get(`${api}/merge-queue/main`, {headers: apiHeaders()})).json();
  let headSHA = '';
  await expect(async () => {
    const pr = await getPull();
    expect(pr.mergeable).toBe(true);
    headSHA = pr.head.sha;
  }).toPass();
  await setStatus(headSHA, headState);
  await page.goto(`/${owner}/${repo}/pulls/${index}`, {waitUntil: 'commit'});

  const addToQueue = async () => {
    const mergeBox = page.locator('#pull-request-merge-form');
    const addButton = {name: 'Add to merge queue', exact: true};
    await mergeBox.getByRole('button', addButton).click();
    await mergeBox.locator('form').getByRole('button', addButton).click();
    let groupSHA = '';
    await expect(async () => {
      const [entry] = await getQueue();
      expect(entry).toMatchObject({pull_request_index: index, state: 'awaiting_checks'});
      groupSHA = entry.merge_group_head_sha;
    }).toPass();
    return groupSHA;
  };
  return {headSHA, setStatus, addToQueue, getPull, getQueue};
}

test('the admin queues by default, can explicitly bypass, and merges after queue checks pass', async ({page, request}) => {
  const {headSHA, setStatus, addToQueue, getPull} = await createQueueRequiredPR(page, request, 'pending');

  const mergeBox = page.locator('#pull-request-merge-form');
  const bypass = 'Create merge commit (bypass merge queue and checks)';
  await mergeBox.locator('.dropdown').click();
  await expect(mergeBox.getByText(bypass, {exact: true})).toBeVisible();
  await mergeBox.getByRole('button', {name: 'Add to merge queue when ready', exact: true}).click();
  await expect(mergeBox.locator('[name="merge_when_checks_succeed"]')).toHaveValue('true');
  await expect(mergeBox.locator('[name="force_merge"]')).toHaveValue('false');

  await setStatus(headSHA, 'success');
  await page.reload({waitUntil: 'commit'});
  await expect(mergeBox.getByRole('button', {name: 'Add to merge queue', exact: true})).toBeVisible();
  await mergeBox.locator('.dropdown').click();
  await mergeBox.getByText(bypass, {exact: true}).click();
  await mergeBox.getByRole('button', {name: bypass}).click();
  await expect(mergeBox.locator('[name="force_merge"]')).toHaveValue('true');
  await mergeBox.getByRole('button', {name: 'Cancel'}).click();
  await mergeBox.locator('.dropdown').click();
  await mergeBox.getByText('Add to merge queue', {exact: true}).click();
  const groupSHA = await addToQueue();

  await page.getByRole('link', {name: 'merge queue of main'}).click();
  await expect(page.getByText('Waiting for ci', {exact: true})).toBeVisible();
  await setStatus(groupSHA, 'success');
  await expect(async () => expect((await getPull()).merged).toBe(true)).toPass();
});

test('failed queue checks remove a pull request without naming the enqueuer', async ({page, request}) => {
  const {setStatus, addToQueue, getQueue} = await createQueueRequiredPR(page, request, 'success');
  await setStatus(await addToQueue(), 'failure');
  await expect(async () => expect(await getQueue()).toEqual([])).toPass();

  await page.reload({waitUntil: 'commit'});
  const removal = page.locator('.timeline-item.event').filter({hasText: 'Removed from the merge queue because the required checks failed'});
  await expect(removal).toBeVisible();
  await expect(removal).not.toContainText(owner);
});
