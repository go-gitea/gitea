import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateBranch, apiCreateOrg, apiCreateRepo, apiSetRepoTopics, randomString} from './utils.ts';

test('create a repository for an organization picked by keyboard', async ({page, request}) => {
  const repoName = `e2e-repo-${randomString(8)}`;
  const orgName = `e2e-repo-org-${randomString(8)}`;
  await Promise.all([apiCreateOrg(request, orgName), login(page)]);
  await page.goto('/repo/create');
  const owner = page.locator('#repo_owner_dropdown').getByRole('button');
  await owner.focus();
  await page.keyboard.press('ArrowDown');
  await page.keyboard.type(orgName);
  await page.keyboard.press('Enter');
  await expect(owner).toContainText(orgName);
  await expect(owner).toBeFocused();
  await page.locator('input[name="repo_name"]').fill(repoName);
  await page.getByRole('button', {name: 'Create Repository'}).click();
  await page.waitForURL(new RegExp(`/${orgName}/${repoName}$`));
});

test('delete a branch via its modal, add and remove repository topics', async ({page, request}) => {
  const repoName = `e2e-branch-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName});
      await Promise.all([
        apiCreateBranch(request, owner, repoName, 'feature'),
        apiSetRepoTopics(request, owner, repoName, ['e2e-drop']),
      ]);
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/branches`);
  await page.getByRole('button', {name: 'Delete Branch "feature"'}).click();
  const modal = page.getByRole('dialog');
  await expect(modal).toContainText('feature');
  await modal.getByRole('button', {name: 'Yes'}).click();
  await expect(page.locator('.flash-message')).toContainText('Branch "feature" has been deleted.');
  await page.goto(`/${owner}/${repoName}`);
  await page.getByRole('button', {name: 'Manage Topics'}).click();
  await page.keyboard.insertText('e2e-keep');
  await page.getByRole('option', {name: 'e2e-keep', exact: true}).click();
  await page.getByRole('button', {name: 'Remove item "e2e-drop"'}).click();
  await page.getByRole('button', {name: 'Save', exact: true}).click();
  await expect(page.locator('#repo-topics').getByRole('link')).toHaveText(['e2e-keep']);
});
