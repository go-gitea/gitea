import {env} from 'node:process';
import {expect} from '@playwright/test';
import type {Page} from '@playwright/test';

export async function login(page: Page) {
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(env.E2E_USER!);
  await page.getByLabel('Password').fill(env.E2E_PASSWORD!);
  await page.getByRole('button', {name: 'Sign In'}).click();
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeHidden();
}

export async function logout(page: Page) {
  await page.getByText('Sign Out').dispatchEvent('click');
  await page.waitForURL('**/');
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeVisible();
}
