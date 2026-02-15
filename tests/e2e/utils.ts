import {expect} from '@playwright/test';
import type {Browser, Page} from '@playwright/test';

const LOGIN_PASSWORD = 'password';

export async function login(page: Page, user: string = 'e2e') {
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(user);
  await page.getByLabel('Password').fill(LOGIN_PASSWORD);
  await page.getByRole('button', {name: 'Sign In'}).click();
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeHidden();
}

export async function logout(page: Page) {
  await page.getByText('Sign Out').dispatchEvent('click');
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeVisible();
}

export async function login_user(browser: Browser, user: string) {
  const context = await browser.newContext();
  const page = await context.newPage();
  await login(page, user);
  return context;
}
