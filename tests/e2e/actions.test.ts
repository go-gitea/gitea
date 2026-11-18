import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateFiles, apiCreateRepo, apiHeaders, login, randomString} from './utils.ts';

test('job queue refreshes with filter open', async ({page, request}) => {
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
  await page.goto(`/${owner}/${repo}/actions/job_queue`);
  await page.getByRole('menu').getByText('Status', {exact: true}).click();
  const waitingFilter = page.getByRole('menuitem', {name: 'Waiting'});
  await expect(waitingFilter).toBeVisible();

  await dispatch();
  await page.clock.fastForward(3000);
  await expect(page.getByRole('link', {name: 'queued #2'})).toBeVisible();
  await expect(waitingFilter).toBeVisible();
});

test('workflow graph opens a job and steps expand by click or log permalink', async ({page}) => {
  await page.goto('/devtest/repo-action-view/runs/10');
  await page.locator('.workflow-graph .job-name').getByText('job-100', {exact: true}).click();
  await page.waitForURL('**/devtest/repo-action-view/runs/10/jobs/10000');
  await page.getByText('step 1 (mock fast)', {exact: true}).click();
  await expect(page.getByText('message for: step=1, cursor=23', {exact: true})).toBeVisible();

  await page.goto('/devtest/repo-action-view/runs/10/jobs/1#jobstep-1-23');
  await expect(page.getByRole('link', {name: '23', exact: true})).toBeInViewport();
});

test('run workflow dispatches a queued run with an input', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `e2e-dispatch-${randomString(8)}`;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repo, autoInit: false});
      await apiCreateFiles(request, owner, repo, [{
        path: '.gitea/workflows/dispatch.yml',
        content: `run-name: Dispatch \${{ inputs.message }}
on:
  workflow_dispatch:
    inputs:
      message:
        description: Run message
jobs:
  queued:
    runs-on: no-runner
    steps:
      - run: exit 0
`,
      }]);
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repo}/actions?workflow=dispatch.yml`);
  await page.getByRole('button', {name: 'Run Workflow', exact: true}).click();
  const dispatchDialog = page.getByRole('dialog');
  await dispatchDialog.getByLabel('Run message').fill('from UI');
  await dispatchDialog.getByRole('button', {name: 'Run Workflow', exact: true}).click();
  await expect(page.getByRole('link', {name: 'Dispatch from UI', exact: true})).toBeVisible();
});
