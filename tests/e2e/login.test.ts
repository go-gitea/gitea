import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {logout} from './utils.ts';

test('homepage', async ({page}) => {
  await page.goto('/');
  await expect(page.getByRole('img', {name: 'Logo'})).toHaveAttribute('src', '/assets/img/logo.svg');
});

test('login form and logout', async ({page}) => {
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(env.GITEA_TEST_E2E_USER);
  await page.getByLabel('Password').fill(env.GITEA_TEST_E2E_PASSWORD);
  await page.getByRole('button', {name: 'Sign In'}).click();
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeHidden();
  await logout(page);
});
