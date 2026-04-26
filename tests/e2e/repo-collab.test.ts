import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateRepo, apiCreateUser, login, randomString} from './utils.ts';

test('add collaborator search', async ({page, request}) => {
  const userName = `repo-collab-${randomString(8)}`;
  const repoName = `repo-collab-${randomString(8)}`;

  await Promise.all([
    apiCreateUser(request, userName),
    apiCreateRepo(request, {name: repoName, autoInit: false}),
    login(page),
  ]);

  await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/settings/collaboration`);
  const input = page.locator('#search-user-box input.prompt');
  await input.fill(userName.slice(-6));
  const result = page.locator('#search-user-box .results .result').first();
  await expect(result).toContainText(userName);
  await result.click();
  await expect(input).toHaveValue(userName);
  await page.getByRole('button', {name: 'Add Collaborator'}).click();
  await expect(page.locator('body')).toContainText(userName);
});
