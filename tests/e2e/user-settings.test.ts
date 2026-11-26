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
    buffer: Buffer.from('R0lGODlhKAAUAPABAAAAAP///yH5BAAAAAAALAAAAAAoABQAAAI2hI+hy+2vkASw2jXT3S4fDmIeFYIjWW5nyq3s5b5VLD903dy46O326OsAgwydz7hD4pQ1pqwAADs=', 'base64'),
  });
  const cropBox = page.locator('image-crop[loaded] .crop-box');
  await cropBox.dragTo(cropBox, {sourcePosition: {x: 100, y: 100}, targetPosition: {x: 20, y: 100}, steps: 4});
  await page.getByRole('button', {name: 'Update Avatar'}).click();
  await expect(page.getByText('Your avatar has been updated.', {exact: true})).toBeVisible();
  const avatar = await page.evaluate(async (src) => {
    const image = new Image();
    image.src = src;
    await image.decode();
    const context = document.createElement('canvas').getContext('2d')!;
    context.drawImage(image, 0, 0);
    return {width: image.naturalWidth, height: image.naturalHeight, pixels: [...context.getImageData(5, 0, 2, 1).data]};
  }, `/user/avatar/${username}/0`);
  expect(avatar).toEqual({width: 20, height: 20, pixels: [0, 0, 0, 255, 255, 255, 255, 255]});
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
