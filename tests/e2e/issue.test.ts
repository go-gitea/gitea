import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateIssue, apiCreateRepo, randomString} from './utils.ts';

test('filter issues by an author and add an issue dependency, both found through remote searches', async ({page, request}) => {
  const repoName = `e2e-issue-dependency-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const blockingTitle = `Blocking ${randomString(8)}`;
  const [blocked] = await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      const [issue] = await Promise.all([
        apiCreateIssue(request, {owner, repo: repoName, title: 'Blocked issue'}),
        apiCreateIssue(request, {owner, repo: repoName, title: blockingTitle}),
      ]);
      return issue;
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/issues`);
  await page.locator('.dropdown-trigger', {hasText: 'Author'}).click();
  await page.keyboard.insertText(owner);
  await page.getByRole('menuitem', {name: owner}).click();
  await page.waitForURL(new RegExp(`[?&]poster=${owner}(&|$)`));
  await expect(page.getByRole('link', {name: 'Blocked issue'})).toBeVisible();
  await page.goto(`/${owner}/${repoName}/issues/${blocked.index}`);
  await page.getByRole('combobox', {name: 'Add dependency…'}).click();
  await page.keyboard.insertText(blockingTitle);
  await page.getByRole('option', {name: blockingTitle}).click();
  await page.locator('#addDependencyForm button[type="submit"]').click();
  await expect(page.locator('.ui.depending').getByRole('link', {name: blockingTitle})).toBeVisible();
});
