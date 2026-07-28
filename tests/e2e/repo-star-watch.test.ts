import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateUser, apiUserHeaders, randomString} from './utils.ts';

test('star and watch a repository', async ({page, request}) => {
  const owner = `sw-owner-${randomString(8)}`;
  const repoName = `e2e-star-watch-${randomString(8)}`;
  await apiCreateUser(request, owner);
  await Promise.all([
    apiCreateRepo(request, {name: repoName, autoInit: false, headers: apiUserHeaders(owner)}),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}`);

  // exact match so "Star"/"Watch" don't also match "Unstar"/"Unwatch"
  await page.getByRole('button', {name: 'Star', exact: true}).click();
  await expect(page.getByRole('button', {name: 'Unstar'})).toBeVisible();

  await page.getByRole('button', {name: 'Watch', exact: true}).click();
  await expect(page.getByRole('button', {name: 'Unwatch'})).toBeVisible();
});
