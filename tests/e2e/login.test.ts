import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {logout} from './utils.ts';

test('homepage', async ({page}) => {
  await page.goto('/');
  await expect(page.getByRole('img', {name: 'Logo'})).toHaveAttribute('src', '/assets/img/logo.svg');
});

test('login form in tab order and logout', async ({page}) => {
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(env.GITEA_TEST_E2E_USER);
  await page.keyboard.press('Tab');
  const password = page.getByLabel('Password');
  await expect(password).toBeFocused();
  await password.fill(env.GITEA_TEST_E2E_PASSWORD);
  await page.keyboard.press('Tab');
  await expect(page.getByLabel('Remember This Device')).toBeFocused();
  await page.keyboard.press('Tab');
  const signIn = page.getByRole('button', {name: 'Sign In', exact: true});
  await expect(signIn).toBeFocused();
  await page.keyboard.press('Tab');
  await expect(page.getByRole('link', {name: 'Forgot password?'})).toBeFocused();
  await signIn.click();
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeHidden();
  await logout(page);
});
