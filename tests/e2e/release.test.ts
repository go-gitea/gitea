import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, randomString} from './utils.ts';

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
