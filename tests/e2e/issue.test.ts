import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateIssue, apiCreateRepo, randomString} from './utils.ts';

test('filter issues by an author and add an issue dependency, both found through remote searches and picked by Enter or leaving the field', async ({page, request}) => {
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
  await expect(page.getByRole('menuitem')).toHaveText([new RegExp(owner)]);
  await page.keyboard.press('Enter');
  await page.waitForURL(new RegExp(`[?&]poster=${owner}(&|$)`));
  await expect(page.getByRole('link', {name: 'Blocked issue'})).toBeVisible();
  await page.goto(`/${owner}/${repoName}/issues/${blocked.index}`);
  const dependency = page.getByRole('combobox', {name: 'Add dependency…'});
  await dependency.click();
  await page.keyboard.insertText(blockingTitle);
  await expect(dependency).toHaveAttribute('aria-activedescendant');
  await page.locator('#addDependencyForm button[type="submit"]').click();
  await expect(page.locator('.ui.depending').getByRole('link', {name: blockingTitle})).toBeVisible();
});
