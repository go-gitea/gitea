import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  apiCreateBranchProtection, apiCreateFiles, apiCreatePR, apiCreateRepo, apiHeaders, assertNoJsError, login, randomString,
} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('auto-merge can be canceled before a squash bypass merge', async ({page, request}) => {
  const repo = `e2e-merge-box-${randomString(8)}`;
  const createPR = (async () => {
    await apiCreateRepo(request, {name: repo});
    await Promise.all([
      apiCreateFiles(request, owner, repo, [{path: 'feat.txt', content: 'feature\n'}], {branch: 'main', newBranch: 'feat'}),
      apiCreateBranchProtection(request, owner, repo, 'main', {required_approvals: 1}),
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
  const commandInstructions = page.getByRole('button', {name: 'View command line instructions.'});
  await expect(commandInstructions).toBeVisible();
  await commandInstructions.press('Enter');
  const commandDialog = page.getByRole('dialog', {name: 'Command line instructions'});
  await expect(commandDialog.getByRole('heading', {name: 'Checkout', exact: true})).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(commandDialog).toBeHidden();
  await page.getByRole('button', {name: 'Disable auto-merge'}).click();
  await expect(page.getByText('disabled auto-merge')).toBeVisible();

  await page.getByRole('button', {name: 'Select merge style'}).click();
  await page.getByRole('menuitemradio', {name: /^Create squash commit/}).click();
  await page.getByLabel('Merge without waiting for requirements to be met (bypass rules)').check();
  await page.getByRole('button', {name: 'Bypass rules and merge (squash)'}).click();
  await page.getByRole('button', {name: 'Confirm bypass rules and merge (squash)'}).click();

  await expect(page.getByText('Merged', {exact: true})).toBeVisible();
  await assertNoJsError(page);
});
