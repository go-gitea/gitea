import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateIssue, apiCreateLabel, apiCreateRepo, randomString} from './utils.ts';

test('filter issues by an author, label them in bulk and add an issue dependency, found through remote searches and picked by Enter or leaving the field', async ({page, request}) => {
  const repoName = `e2e-issue-dependency-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const blockingTitle = `Blocking ${randomString(8)}`;
  const [blocked] = await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      const [issue] = await Promise.all([
        apiCreateIssue(request, {owner, repo: repoName, title: 'Blocked issue'}),
        apiCreateIssue(request, {owner, repo: repoName, title: blockingTitle}),
        apiCreateLabel(request, owner, repoName, 'bulk', '#ff0000'),
      ]);
      return issue;
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/issues`);
  await page.getByRole('button').filter({hasText: 'Author'}).click();
  await page.keyboard.insertText(owner);
  await expect(page.getByRole('menuitem')).toHaveText([new RegExp(owner)]);
  await page.keyboard.press('Enter');
  await page.waitForURL(new RegExp(`[?&]poster=${owner}(&|$)`));
  await expect(page.getByRole('link', {name: 'Blocked issue'})).toBeVisible();
  await page.getByRole('checkbox', {name: 'Check/Uncheck all items'}).check();
  await page.getByRole('button', {name: 'Label', exact: true}).click();
  await page.getByRole('menuitem', {name: 'bulk'}).click();
  await expect(page.locator('#issue-list').getByRole('link', {name: 'bulk'})).toHaveCount(2);
  await page.goto(`/${owner}/${repoName}/issues/${blocked.index}`);
  const dependency = page.getByRole('combobox', {name: 'Add dependency…'});
  await dependency.click();
  await page.keyboard.insertText(blockingTitle);
  await expect(dependency).toHaveAttribute('aria-activedescendant');
  await page.locator('#addDependencyForm button[type="submit"]').click();
  await expect(page.locator('.ui.depending').getByRole('link', {name: blockingTitle})).toBeVisible();
});
