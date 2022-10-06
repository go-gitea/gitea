// @ts-check
import {test, expect} from '@playwright/test';
import {login_user, save_visual, load_logged_in_context} from './utils_e2e.js';

/*
test.beforeAll(async ({browser}, workerInfo) => {
  await login_user(browser, workerInfo, 'user2');
});
*/

test('Load Homepage', async ({page}) => {
  // await login_user(browser, workerInfo, 'user2');
  const response = await page.goto('/user/login');
  await expect(response?.status()).toBe(200); // Status OK
});
