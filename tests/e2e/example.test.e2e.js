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
  await page.type('input[name=user_name]', `e2e-test-${workerInfo.workerIndex}`);
  await page.type('input[name=email]', `e2e-test-${workerInfo.workerIndex}@test.com`);
  await page.type('input[name=password]', 'test123test123');
  await page.type('input[name=retype]', 'test123test123');
  await page.click('form button.ui.primary.button:visible');
  // Make sure we routed to the home page. Else login failed.
  await expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);
  await expect(page.locator('.dashboard-navbar span>img.ui.avatar')).toBeVisible();
  await expect(page.locator('.ui.positive.message.flash-success')).toHaveText('Account was successfully created. Welcome!');

  save_visual(page);
});

test('Test Login Form', async ({page}, workerInfo) => {
  const response = await page.goto('/user/login');
  await expect(response?.status()).toBe(200); // Status OK

  await page.type('input[name=user_name]', `user2`);
  await page.type('input[name=password]', `password`);
  await page.click('form button.ui.primary.button:visible');

  await page.waitForLoadState('networkidle');

  await expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);

  save_visual(page);
});

test('Test Logged In User', async ({browser}, workerInfo) => {
  const context = await load_logged_in_context(browser, workerInfo, 'user2');
  const page = await context.newPage();

  await page.goto('/');

  // Make sure we routed to the home page. Else login failed.
  await expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);

  save_visual(page);
});
