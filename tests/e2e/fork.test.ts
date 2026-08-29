import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateUser, apiUserHeaders, randomString} from './utils.ts';

test('fork a repository', async ({page, request}) => {
  const upstream = `fork-owner-${randomString(8)}`;
  const repoName = `e2e-fork-${randomString(8)}`;
  await apiCreateUser(request, upstream);
  await Promise.all([
    apiCreateRepo(request, {name: repoName, headers: apiUserHeaders(upstream)}),
    login(page),
  ]);
  await page.goto(`/${upstream}/${repoName}/fork`);

  await page.getByRole('button', {name: 'Fork Repository'}).click();
  await page.waitForURL(new RegExp(`/${env.GITEA_TEST_E2E_USER}/${repoName}$`));
  await expect(page.getByRole('link', {name: `${upstream}/${repoName}`})).toBeVisible();
});
