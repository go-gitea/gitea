import {test, expect} from '@playwright/test';
import {login} from './utils.ts';

test('update profile biography', async ({page}) => {
  const bio = `e2e-bio-${Date.now()}`;
  await login(page);
  await page.goto('/user/settings');
  await page.getByLabel('Biography').fill(bio);
  await page.getByRole('button', {name: 'Update Profile'}).click();
  await expect(page.getByLabel('Biography')).toHaveValue(bio);
  await page.getByLabel('Biography').fill('');
  await page.getByRole('button', {name: 'Update Profile'}).click();
  await expect(page.getByLabel('Biography')).toHaveValue('');
});
