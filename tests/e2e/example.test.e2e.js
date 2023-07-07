// @ts-check
import {test, expect} from '@playwright/test';
import {login_user, save_visual, load_logged_in_context} from './utils_e2e.js';

test.beforeAll(async ({browser}, workerInfo) => {
  await login_user(browser, workerInfo, 'user2');
});

test('Load Homepage', async ({page}) => {
  const response = await page.goto('/');
  await expect(response?.status()).toBe(200); // Status OK
  await expect(page).toHaveTitle(/^Gitea: Git with a cup of tea\s*$/);
  await expect(page.locator('.logo')).toHaveAttribute('src', '/assets/img/logo.svg');
});

test('Test Register Form', async ({page}, workerInfo) => {
  const response = await page.goto('/user/sign_up');
  await expect(response?.status()).toBe(200); // Status OK
  await page.locator('input#user_name').fill(`e2e-test-${workerInfo.workerIndex}`);
  await page.locator('input#email').fill(`e2e-test-${workerInfo.workerIndex}@test.com`);
  await page.locator('input#password').fill('test123');
  await page.locator('input#retype').fill('test123');
  await page.locator('form button.ui.green.button:visible').click();
  // Make sure we routed to the home page. Else login failed.
  await expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);
  await expect(page.locator('.dashboard-navbar span>img.ui.avatar')).toBeVisible();
  await expect(page.locator('.ui.positive.message.flash-success')).toHaveText('Account was successfully created.');

  await save_visual(page);
});

test('Test Login Form', async ({page}, workerInfo) => {
  const response = await page.goto('/user/login');
  await expect(response?.status()).toBe(200); // Status OK

  await page.locator('input#user_name').fill('user2');
  await page.locator('input#password').fill('password');
  await page.locator('form button.ui.green.button:visible').click();

  await expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);

  await save_visual(page);
});

test('Test Logged In User', async ({browser}, workerInfo) => {
  const context = await load_logged_in_context(browser, workerInfo, 'user2');
  const page = await context.newPage();

  await page.goto('/');

  // Make sure we routed to the home page. Else login failed.
  await expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);

  await save_visual(page);
});
