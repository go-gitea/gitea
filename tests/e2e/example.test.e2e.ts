import {test, expect} from '@playwright/test';
import {login_user, save_visual, load_logged_in_context} from './utils_e2e.ts';

test.beforeAll(async ({browser}, workerInfo) => {
  await login_user(browser, workerInfo, 'user2');
});

test('homepage', async ({page}) => {
  const response = await page.goto('/');
  expect(response?.status()).toBe(200); // Status OK
  await expect(page).toHaveTitle(/^Gitea: Git with a cup of tea\s*$/);
  await expect(page.locator('.logo')).toHaveAttribute('src', '/assets/img/logo.svg');
});

test('register', async ({page}, workerInfo) => {
  const response = await page.goto('/user/sign_up');
  expect(response?.status()).toBe(200); // Status OK
  await page.locator('input[name=user_name]').fill(`e2e-test-${workerInfo.workerIndex}`);
  await page.locator('input[name=email]').fill(`e2e-test-${workerInfo.workerIndex}@test.com`);
  await page.locator('input[name=password]').fill('test123test123');
  await page.locator('input[name=retype]').fill('test123test123');
  await page.click('form button.ui.primary.button:visible');
  // Make sure we routed to the home page. Else login failed.
  expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);
  await expect(page.locator('.secondary-nav span>img.ui.avatar')).toBeVisible();
  await expect(page.locator('.ui.positive.message.flash-success')).toHaveText('Account was successfully created. Welcome!');

  save_visual(page);
});

test('login', async ({page}, workerInfo) => {
  const response = await page.goto('/user/login');
  expect(response?.status()).toBe(200); // Status OK

  await page.locator('input[name=user_name]').fill(`user2`);
  await page.locator('input[name=password]').fill(`password`);
  await page.click('form button.ui.primary.button:visible');

  await page.waitForLoadState('networkidle'); // eslint-disable-line playwright/no-networkidle

  expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);

  save_visual(page);
});

test('logged in user', async ({browser}, workerInfo) => {
  const context = await load_logged_in_context(browser, workerInfo, 'user2');
  const page = await context.newPage();

  await page.goto('/');

  // Make sure we routed to the home page. Else login failed.
  expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);

  save_visual(page);
});
