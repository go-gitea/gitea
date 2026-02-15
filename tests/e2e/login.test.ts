import {test, expect} from '@playwright/test';
import {login, logout} from './utils.ts';

test('homepage', async ({page}) => {
  await page.goto('/');
  await expect(page.getByRole('img', {name: 'Logo'})).toHaveAttribute('src', '/assets/img/logo.svg');
});

test('login and logout', async ({page}) => { // eslint-disable-line playwright/expect-expect
  await login(page);
  await logout(page);
});
