import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiBaseUrl, apiHeaders, apiCreateRepo, apiDeleteRepo, apiCreateBranch, apiCreatePullRequest, apiSetCommitStatus, apiSetBranchProtection} from '../utils.ts';

async function createPullRequestWithFile(requestContext, repoName: string): Promise<{number: number; head_sha: string}> {
  await apiCreateBranch(requestContext, env.GITEA_TEST_E2E_USER, repoName, 'test-branch');
  await requestContext.post(`${apiBaseUrl()}/api/v1/repos/${env.GITEA_TEST_E2E_USER}/${repoName}/contents/branch-file.txt`, {
    headers: apiHeaders(),
    data: {content: globalThis.btoa('branch content'), branch: 'test-branch'},
  });
  return apiCreatePullRequest(requestContext, env.GITEA_TEST_E2E_USER, repoName, {
    title: 'Test PR for merge box',
    head: 'test-branch',
  });
}

test.describe('pull merge box', () => {
  test('style switching and form fields', async ({page}) => {
    const repoName = `e2e-merge-box-${Date.now()}`;
    await login(page);
    await apiCreateRepo(page.request, {name: repoName});
    const pr = await createPullRequestWithFile(page.request, repoName);
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${pr.number}`);

    const mergeBox = page.locator('.pull-merge-box');
    const mergeButton = mergeBox.locator('.merge-button');
    await expect(mergeButton).toBeVisible();
    await expect(mergeButton).toContainText('Create merge commit');

    // open dropdown and switch to squash
    await mergeButton.locator('.ui.dropdown').click();
    const menu = mergeButton.locator('.menu');
    await expect(menu).toBeVisible();
    await menu.getByText('Create squash commit').click();
    await expect(mergeButton).toContainText('Create squash commit');

    // expand form — squash should show title/message fields with correct defaults
    await mergeButton.locator('button.ui.button').first().click();
    const form = mergeBox.locator('form');
    await expect(form.locator('input[name="merge_title_field"]')).toHaveValue(`Test PR for merge box (#${pr.number})`);
    await expect(form.locator('textarea[name="merge_message_field"]')).toHaveValue(/Reviewed-on:/);
    await expect(form.getByRole('button', {name: /Create squash commit/})).toBeVisible();

    // delete branch checkbox should be present
    await expect(mergeBox.locator('#delete-branch-after-merge')).toBeVisible();

    // cancel and switch to rebase — should hide title/message fields
    await form.getByRole('button', {name: 'Cancel'}).click();
    await mergeButton.locator('.ui.dropdown').click();
    await menu.getByText('Rebase, then fast-forward').click();
    await expect(mergeButton).toContainText('Rebase, then fast-forward');
    await mergeButton.locator('button.ui.button').first().click();
    await expect(mergeBox.locator('form input[name="merge_title_field"]')).toBeHidden();
    await expect(mergeBox.locator('form textarea[name="merge_message_field"]')).toBeHidden();

    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  });

  test('status checks and auto-merge', async ({page}) => {
    const repoName = `e2e-merge-box-${Date.now()}`;
    await login(page);
    await apiCreateRepo(page.request, {name: repoName});
    const pr = await createPullRequestWithFile(page.request, repoName);
    await apiSetBranchProtection(page.request, env.GITEA_TEST_E2E_USER, repoName, 'main', {
      statusCheckContexts: ['ci/test'],
    });

    const mergeBox = page.locator('.pull-merge-box');
    const mergeButton = mergeBox.locator('.merge-button');
    const prUrl = `/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${pr.number}`;

    // success → green primary button
    await apiSetCommitStatus(page.request, env.GITEA_TEST_E2E_USER, repoName, pr.head_sha, {
      context: 'ci/test', state: 'success', description: 'Passed',
    });
    await page.goto(prUrl);
    await expect(mergeButton).toHaveClass(/primary/);
    await expect(mergeBox).toContainText('can be merged automatically');

    // failure → red button with admin override
    await apiSetCommitStatus(page.request, env.GITEA_TEST_E2E_USER, repoName, pr.head_sha, {
      context: 'ci/test', state: 'failure', description: 'Failed',
    });
    await page.goto(prUrl);
    await expect(mergeButton).toHaveClass(/red/);
    await expect(mergeBox).toContainText('As an administrator');

    // pending → auto-merge option in dropdown
    await apiSetCommitStatus(page.request, env.GITEA_TEST_E2E_USER, repoName, pr.head_sha, {
      context: 'ci/test', state: 'pending', description: 'Running',
    });
    await page.goto(prUrl);
    await mergeButton.locator('.ui.dropdown').click();
    await expect(mergeButton.locator('.menu')).toContainText(/Auto merge when all checks succeed/);

    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  });

  test('merge and show merged', async ({page}) => {
    const repoName = `e2e-merge-box-${Date.now()}`;
    await login(page);
    await apiCreateRepo(page.request, {name: repoName});
    const pr = await createPullRequestWithFile(page.request, repoName);
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/pulls/${pr.number}`);

    const mergeBox = page.locator('.pull-merge-box');
    const mergeButton = mergeBox.locator('.merge-button');
    await expect(mergeButton).toBeVisible();

    // expand form and submit merge
    await mergeButton.locator('button.ui.button').first().click();
    await mergeBox.locator('form').getByRole('button', {name: /Create merge commit/}).click();

    // after merge, page should show merged state
    await expect(mergeBox).toContainText(/Merged/i);

    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  });
});
