import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateIssue, apiCreateRepo, randomString} from './utils.ts';

test('filter issues by an author found through the user search', async ({page, request}) => {
  const repoName = `e2e-issue-author-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      await apiCreateIssue(request, {owner, repo: repoName, title: 'Authored issue'});
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/issues`);
  await page.locator('.dropdown-trigger', {hasText: 'Author'}).click();
  await page.keyboard.insertText(owner);
  await page.getByRole('menuitem', {name: owner}).click();
  await page.waitForURL(new RegExp(`[?&]poster=${owner}(&|$)`));
  await expect(page.getByRole('link', {name: 'Authored issue'})).toBeVisible();
});

test('add an issue dependency found through the sidebar search', async ({page, request}) => {
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
  await page.goto(`/${owner}/${repoName}/issues/${blocked.index}`);
  await page.getByRole('combobox', {name: 'Add dependency…'}).click();
  await page.keyboard.insertText(blockingTitle);
  await page.getByRole('option', {name: blockingTitle}).click();
  await page.locator('#addDependencyForm button[type="submit"]').click();
  await expect(page.locator('.ui.depending').getByRole('link', {name: blockingTitle})).toBeVisible();
});
