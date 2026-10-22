import {test, expect} from '@playwright/test';
import {login, loginUser, apiCreateUser, randomString} from './utils.ts';

test('update profile biography', async ({page, request}) => {
  const username = `e2e-settings-${randomString(8)}`;
  const bio = `e2e-bio-${randomString(8)}`;
  await apiCreateUser(request, username);
  await loginUser(page, username);
  await page.goto('/user/settings');
  await page.getByLabel('Biography').fill(bio);
  await page.getByRole('button', {name: 'Update Profile'}).click();
  await expect(page.getByLabel('Biography')).toHaveValue(bio);
  await page.getByLabel('Biography').fill('');
  await page.getByRole('button', {name: 'Update Profile'}).click();
  await expect(page.getByLabel('Biography')).toHaveValue('');
});

test('select and deselect all webhook events', async ({page}) => {
  await login(page);
  await page.goto('/user/settings/hooks/gitea/new');
  await page.getByRole('radio', {name: 'Custom Events'}).check();
  await page.getByRole('button', {name: 'Select All', exact: true}).click();
  await expect(page.getByRole('checkbox', {checked: false})).toHaveCount(0);
  await page.getByRole('button', {name: 'Deselect All', exact: true}).click();
  await expect(page.getByRole('checkbox', {checked: true})).toHaveAccessibleName('Active');
});
