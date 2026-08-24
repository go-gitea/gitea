import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiHeaders, baseUrl, randomString} from './utils.ts';

test('create a release', async ({page, request}) => {
  const repoName = `e2e-release-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([apiCreateRepo(request, {name: repoName}), login(page)]);
  await page.goto(`/${owner}/${repoName}/releases/new`);

  const tag = `v1.0.0-${randomString(8)}`;
  const title = `e2e-release-${randomString(8)}`;
  await page.getByLabel('Tag name').fill(tag);
  await page.getByLabel('Release title').fill(title);
  await page.getByRole('button', {name: 'Publish Release'}).click();

  await page.waitForURL(new RegExp(`/${owner}/${repoName}/releases$`));
  await expect(page.locator('.release-list-title')).toContainText(title);
});

test('immutable release locks its assets', async ({page, request}) => {
  const repoName = `e2e-immutable-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const tag = `v1.0.0-${randomString(8)}`;
  await Promise.all([apiCreateRepo(request, {name: repoName}), login(page)]);
  const api = `${baseUrl()}/api/v1/repos/${owner}/${repoName}`;
  await request.patch(api, {headers: apiHeaders(), data: {immutable_releases: true}});
  const release = await request.post(`${api}/releases`, {headers: apiHeaders(), data: {tag_name: tag, name: tag}});
  expect((await release.json()).immutable).toBe(true);
  await page.goto(`/${owner}/${repoName}/releases/edit/${tag}`);
  await expect(page.getByText('You cannot change the tag, target or assets of a published immutable release.')).toBeVisible();
  await expect(page.locator('.dropzone')).toHaveCount(0);
});
