import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, randomString} from './utils.ts';

test('leaving a new issue with an unsaved description asks for confirmation', async ({page, request}) => {
  const repoName = `e2e-are-you-sure-${randomString(8)}`;
  await Promise.all([apiCreateRepo(request, {name: repoName, autoInit: false}), login(page)]);
  await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/new`);
  await page.getByPlaceholder('Leave a comment').press('a');
  const dialogPromise = page.waitForEvent('dialog');
  page.once('dialog', (dialog) => dialog.dismiss());
  await page.getByRole('link', {name: 'Dashboard'}).click();
  expect((await dialogPromise).type()).toBe('beforeunload');
  await expect(page).toHaveURL(/\/issues\/new$/);
});
