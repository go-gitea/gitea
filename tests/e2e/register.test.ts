import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {logout} from './utils.ts';

test.beforeEach(async ({page}) => {
  await page.goto('/user/sign_up');
});

test('register page has form', async ({page}) => {
  await expect(page.getByLabel('Username')).toBeVisible();
  await expect(page.getByLabel('Email Address')).toBeVisible();
  await expect(page.getByLabel('Password', {exact: true})).toBeVisible();
  await expect(page.getByLabel('Confirm Password')).toBeVisible();
  await expect(page.getByRole('button', {name: 'Register Account'})).toBeVisible();
});

test('register with empty fields shows error', async ({page}) => {
  // HTML5 required attribute prevents submission, so verify the fields are required
  await expect(page.locator('input[name="user_name"][required]')).toBeVisible();
  await expect(page.locator('input[name="email"][required]')).toBeVisible();
  await expect(page.locator('input[name="password"][required]')).toBeVisible();
  await expect(page.locator('input[name="retype"][required]')).toBeVisible();
});

test('register with mismatched passwords shows error', async ({page}) => {
  await page.getByLabel('Username').fill('e2e-register-mismatch');
  await page.getByLabel('Email Address').fill('e2e-register-mismatch@test.gitea.io');
  await page.getByLabel('Password', {exact: true}).fill('password123!');
  await page.getByLabel('Confirm Password').fill('different123!');
  await page.getByRole('button', {name: 'Register Account'}).click();
  await expect(page.locator('.ui.negative.message')).toBeVisible();
});

test('register then login', async ({page}) => {
  const username = `e2e-register-${Date.now()}`;
  const email = `${username}@test.gitea.io`;
  const password = 'password123!';

  await page.getByLabel('Username').fill(username);
  await page.getByLabel('Email Address').fill(email);
  await page.getByLabel('Password', {exact: true}).fill(password);
  await page.getByLabel('Confirm Password').fill(password);
  await page.getByRole('button', {name: 'Register Account'}).click();

  // After successful registration, should be redirected away from sign_up
  await expect(page).not.toHaveURL(/sign_up/);

  // Logout then login with the newly created account
  await logout(page);
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(username);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', {name: 'Sign In'}).click();
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeHidden();

  // Clean up: delete the user via API using the main e2e admin account
  const response = await page.request.delete(`/api/v1/admin/users/${username}?purge=true`, {
    headers: {Authorization: `Basic ${btoa(`${env.E2E_USER}:${env.E2E_PASSWORD}`)}`},
  });
  expect(response.ok()).toBeTruthy();
});

test('register with existing username shows error', async ({page}) => {
  await page.getByLabel('Username').fill('e2e');
  await page.getByLabel('Email Address').fill('e2e-duplicate@test.gitea.io');
  await page.getByLabel('Password', {exact: true}).fill('password123!');
  await page.getByLabel('Confirm Password').fill('password123!');
  await page.getByRole('button', {name: 'Register Account'}).click();
  await expect(page.locator('.ui.negative.message')).toBeVisible();
});

test('sign in link exists', async ({page}) => {
  const signInLink = page.getByText('Sign in now!');
  await expect(signInLink).toBeVisible();
  await signInLink.click();
  await expect(page).toHaveURL(/\/user\/login$/);
});
