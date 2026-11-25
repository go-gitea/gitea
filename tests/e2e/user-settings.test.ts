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
  await expect(page.getByText('Your profile has been updated.', {exact: true})).toBeVisible();
  await expect(page.getByLabel('Biography')).toHaveText(bio);
  await page.getByLabel('Biography').fill('');
  await page.getByRole('button', {name: 'Update Profile'}).click();
  await expect(page.getByText('Your profile has been updated.', {exact: true})).toBeVisible();
  await expect(page.getByLabel('Biography')).toHaveText('');
});

test('crop avatar before upload', async ({page, request}) => {
  const username = `e2e-avatar-${randomString(8)}`;
  await apiCreateUser(request, username);
  await loginUser(page, username);
  await page.goto('/user/settings');
  await expect(page.locator('image-crop:defined')).toBeAttached();
  await page.getByLabel('Choose new avatar').setInputFiles({
    name: 'avatar.gif',
    mimeType: 'image/gif',
    buffer: Buffer.from('R0lGODlhKAAUAPAAAEGDxAAAACH5BAAAAAAALAAAAAAoABQAAAIahI+py+0Po5y02ouz3rz7D4biSJbmiabqihUAOw==', 'base64'),
  });
  const cropBox = page.locator('image-crop[loaded] .crop-box');
  await cropBox.dragTo(cropBox, {sourcePosition: {x: 100, y: 100}, targetPosition: {x: 20, y: 100}, steps: 4});
  await page.getByRole('button', {name: 'Update Avatar'}).click();
  await expect(page.getByText('Your avatar has been updated.', {exact: true})).toBeVisible();
  expect((await request.get(`/user/avatar/${username}/0`)).headers()['content-type']).toBe('image/png');
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
