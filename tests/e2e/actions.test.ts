import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateFiles, apiCreateRepo, apiHeaders, randomString} from './utils.ts';

test('job queue refreshes its rows while the filter dropdown stays open', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `e2e-queue-${randomString(8)}`;
  await apiCreateRepo(request, {name: repo, autoInit: false});
  await apiCreateFiles(request, owner, repo, [{
    path: '.gitea/workflows/queue.yml',
    content: 'on: workflow_dispatch\njobs:\n  queued:\n    runs-on: no-runner\n    steps:\n      - run: exit 0\n',
  }]);
  const dispatch = async () => {
    const response = await request.post(`/api/v1/repos/${owner}/${repo}/actions/workflows/queue.yml/dispatches`, {headers: apiHeaders(), data: {ref: 'main'}});
    expect(response.ok()).toBe(true);
  };
  await dispatch();

  await page.clock.install();
  await page.goto(`/${owner}/${repo}/actions/queue`);
  await page.locator('#actions-queue-filter').getByText('Status', {exact: true}).click();
  const waitingFilter = page.getByRole('menuitem', {name: 'Waiting'});
  await expect(waitingFilter).toBeVisible();

  await dispatch();
  await page.clock.fastForward(3000);
  await expect(page.getByRole('link', {name: 'queued #2'})).toBeVisible();
  await expect(waitingFilter).toBeVisible();
});
