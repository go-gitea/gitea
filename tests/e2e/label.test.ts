import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, randomString} from './utils.ts';

test('create a label in the modal with a color from the picker popup', async ({page}) => {
  const repoName = `e2e-label-${randomString(8)}`;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName, autoInit: false})]);
  await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/labels`);
  await page.getByRole('button', {name: 'New Label'}).click();
  const modal = page.getByRole('dialog');
  await modal.getByRole('button', {name: 'Save'}).click();
  await modal.getByPlaceholder('Label name').fill('e2e-label');
  const colorInput = modal.locator('.color-picker-combo input');
  await colorInput.fill('');
  await page.locator('hex-color-picker').click({position: {x: 10, y: 10}});
  await expect(colorInput).toHaveValue(/^#[0-9a-f]{6}$/);
  await modal.getByRole('button', {name: 'Save'}).click();
  await expect(page.locator('.issue-label-list')).toContainText('e2e-label');
});
