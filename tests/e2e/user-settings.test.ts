import {test, expect} from '@playwright/test';
import {loginUser, apiCreateUser, apiDeleteUser, randomString} from './utils.ts';

test('update profile biography', async ({page, request}) => {
  const username = `e2e-settings-${randomString(8)}`;
  const bio = `e2e-bio-${randomString(8)}`;
  await apiCreateUser(request, username);
  try {
    await loginUser(page, username);
    await page.goto('/user/settings');
    await page.getByLabel('Biography').fill(bio);
    await page.getByRole('button', {name: 'Update Profile'}).click();
    await expect(page.getByLabel('Biography')).toHaveValue(bio);
    await page.getByLabel('Biography').fill('');
    await page.getByRole('button', {name: 'Update Profile'}).click();
    await expect(page.getByLabel('Biography')).toHaveValue('');
  } finally {
    await apiDeleteUser(request, username);
  }
});
