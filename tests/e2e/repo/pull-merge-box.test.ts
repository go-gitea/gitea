import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo, apiCreateFile, apiCreateBranch, apiCreatePullRequest, apiSetCommitStatus, apiSetBranchProtection} from '../utils.ts';

test.describe('Pull request merge box', () => {
  let repoName: string;
  let prNumber: number;
  let headSha: string;

  test.beforeEach(async ({page}) => {
    repoName = `e2e-merge-box-${Date.now()}`;
    await login(page);

    // create repo, branch, file on branch, then PR
    await apiCreateRepo(page.request, {name: repoName});
    await apiCreateBranch(page.request, env.GITEA_TEST_E2E_USER, repoName, 'test-branch');
    await apiCreateFile(page.request, env.GITEA_TEST_E2E_USER, repoName, 'test-file.txt', 'test content');

    // create file on branch to make a diff
    await page.request.post(`${env.GITEA_TEST_E2E_URL?.replace(/\/$/g, '')}/api/v1/repos/${env.GITEA_TEST_E2E_USER}/${repoName}/contents/branch-file.txt`, {
      headers: {Authorization: `Basic ${globalThis.btoa(`${env.GITEA_TEST_E2E_USER}:${env.GITEA_TEST_E2E_PASSWORD}`)}`},
      data: {content: globalThis.btoa('branch content'), branch: 'test-branch'},
    });

    const pr = await apiCreatePullRequest(page.request, env.GITEA_TEST_E2E_USER, repoName, {
      title: 'Test PR for merge box',
      head: 'test-branch',
    });
    prNumber = pr.number;
    headSha = pr.head_sha;
  });

  test.afterEach(async ({page}) => {
    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  });

  test('merge style switching', async ({page}) => {
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${prNumber}`);

    const mergeBox = page.locator('.pull-merge-box');
    await expect(mergeBox).toBeVisible();

    // should show merge form with merge button
    const mergeButton = mergeBox.locator('.merge-button');
    await expect(mergeButton).toBeVisible();
    await expect(mergeButton).toContainText('Create merge commit');

    // open dropdown and switch to squash
    await mergeButton.locator('.ui.dropdown').click();
    const menu = mergeButton.locator('.menu');
    await expect(menu).toBeVisible();
    await menu.getByText('Create squash commit').click();
    await expect(mergeButton).toContainText('Create squash commit');

    // expand form and verify squash-specific fields
    await mergeButton.locator('button.ui.button').first().click();
    const form = mergeBox.locator('form');
    await expect(form.locator('input[name="merge_title_field"]')).toBeVisible();
    await expect(form.locator('textarea[name="merge_message_field"]')).toBeVisible();
    await expect(form.getByRole('button', {name: /Create squash commit/})).toBeVisible();

    // cancel and switch to rebase (should hide message fields)
    await form.getByRole('button', {name: 'Cancel'}).click();
    await mergeButton.locator('.ui.dropdown').click();
    await menu.getByText('Rebase, then fast-forward').click();
    await expect(mergeButton).toContainText('Rebase, then fast-forward');

    // expand form — rebase should hide title/message fields
    await mergeButton.locator('button.ui.button').first().click();
    await expect(mergeBox.locator('form input[name="merge_title_field"]')).toBeHidden();
    await expect(mergeBox.locator('form textarea[name="merge_message_field"]')).toBeHidden();
  });

  test('status check affects merge form', async ({page}) => {
    // set up branch protection with required status check
    await apiSetBranchProtection(page.request, env.GITEA_TEST_E2E_USER, repoName, 'main', {
      statusCheckContexts: ['ci/test'],
    });

    // set status to success first
    await apiSetCommitStatus(page.request, env.GITEA_TEST_E2E_USER, repoName, headSha, {
      context: 'ci/test',
      state: 'success',
      description: 'All checks passed',
    });

    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${prNumber}`);

    const mergeBox = page.locator('.pull-merge-box');
    await expect(mergeBox).toBeVisible();

    // with all checks passing, merge button should be green (primary)
    const mergeButton = mergeBox.locator('.merge-button');
    await expect(mergeButton).toHaveClass(/primary/);
    await expect(mergeBox).toContainText('can be merged automatically');

    // now fail the required check
    await apiSetCommitStatus(page.request, env.GITEA_TEST_E2E_USER, repoName, headSha, {
      context: 'ci/test',
      state: 'failure',
      description: 'Tests failed',
    });

    // reload and verify the merge form reflects the failure
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${prNumber}`);
    await expect(mergeBox).toBeVisible();
    await expect(mergeBox).toContainText('required checks');

    // admin can still merge — button should be red (override)
    await expect(mergeButton).toHaveClass(/red/);
    await expect(mergeBox).toContainText('As an administrator');

    // restore to success and verify recovery
    await apiSetCommitStatus(page.request, env.GITEA_TEST_E2E_USER, repoName, headSha, {
      context: 'ci/test',
      state: 'success',
      description: 'All checks passed',
    });

    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${prNumber}`);
    await expect(mergeBox).toBeVisible();
    await expect(mergeButton).toHaveClass(/primary/);
  });

  test('pending status check shows auto-merge option in dropdown', async ({page}) => {
    // set up branch protection with required status check
    await apiSetBranchProtection(page.request, env.GITEA_TEST_E2E_USER, repoName, 'main', {
      statusCheckContexts: ['ci/test'],
    });

    // set status to pending
    await apiSetCommitStatus(page.request, env.GITEA_TEST_E2E_USER, repoName, headSha, {
      context: 'ci/test',
      state: 'pending',
      description: 'Waiting for checks',
    });

    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${prNumber}`);

    const mergeBox = page.locator('.pull-merge-box');
    await expect(mergeBox).toBeVisible();

    // admin can still merge with pending checks, but auto-merge option should be in dropdown
    const mergeButton = mergeBox.locator('.merge-button');
    await mergeButton.locator('.ui.dropdown').click();
    const menu = mergeButton.locator('.menu');
    await expect(menu).toBeVisible();
    await expect(menu).toContainText(/Auto merge when all checks succeed/);
  });

  test('squash merge default message contains PR title and commit info', async ({page}) => {
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${prNumber}`);

    const mergeBox = page.locator('.pull-merge-box');
    const mergeButton = mergeBox.locator('.merge-button');

    // switch to squash
    await mergeButton.locator('.ui.dropdown').click();
    await mergeButton.locator('.menu').getByText('Create squash commit').click();

    // expand form
    await mergeButton.locator('button.ui.button').first().click();

    // squash title should contain PR title and number
    const titleField = mergeBox.locator('form input[name="merge_title_field"]');
    await expect(titleField).toHaveValue(`Test PR for merge box (#${prNumber})`);

    // squash message should contain Reviewed-on URL
    const messageField = mergeBox.locator('form textarea[name="merge_message_field"]');
    await expect(messageField).toHaveValue(/Reviewed-on:/);
  });

  test('perform actual merge', async ({page}) => {
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${prNumber}`);

    const mergeBox = page.locator('.pull-merge-box');
    const mergeButton = mergeBox.locator('.merge-button');
    await expect(mergeButton).toBeVisible();

    // expand form and submit merge
    await mergeButton.locator('button.ui.button').first().click();
    const form = mergeBox.locator('form');
    await form.getByRole('button', {name: /Create merge commit/}).click();

    // after merge, page should show merged state
    await expect(page.locator('.pull-merge-box')).toContainText(/Merged/i);
  });

  test('merge box reload preserves functionality', async ({page}) => {
    // set up branch protection with required status check to trigger the reloading interval
    await apiSetBranchProtection(page.request, env.GITEA_TEST_E2E_USER, repoName, 'main', {
      statusCheckContexts: ['ci/test'],
    });
    await apiSetCommitStatus(page.request, env.GITEA_TEST_E2E_USER, repoName, headSha, {
      context: 'ci/test',
      state: 'pending',
      description: 'Running',
    });

    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${prNumber}`);

    const mergeBox = page.locator('.pull-merge-box');
    await expect(mergeBox).toBeVisible();

    // change status to success — the merge box reloads in dev mode (1ms interval)
    await apiSetCommitStatus(page.request, env.GITEA_TEST_E2E_USER, repoName, headSha, {
      context: 'ci/test',
      state: 'success',
      description: 'All checks passed',
    });

    // wait for the merge box to reload and reflect the new status
    await expect(mergeBox.locator('.merge-button')).toHaveClass(/primary/, {timeout: 6000});

    // verify the Vue component still works after reload — open dropdown
    const mergeButton = mergeBox.locator('.merge-button');
    await mergeButton.locator('.ui.dropdown').click();
    await expect(mergeButton.locator('.menu')).toBeVisible();
  });

  test('delete branch checkbox', async ({page}) => {
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${prNumber}`);

    const mergeBox = page.locator('.pull-merge-box');
    const mergeButton = mergeBox.locator('.merge-button');
    await expect(mergeButton).toBeVisible();

    // expand merge form
    await mergeButton.locator('button.ui.button').first().click();

    // verify delete branch checkbox exists
    const deleteBranchCheckbox = mergeBox.locator('#delete-branch-after-merge');
    await expect(deleteBranchCheckbox).toBeVisible();
    await expect(mergeBox).toContainText('Delete Branch');
  });
});
