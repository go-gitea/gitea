import {env} from 'node:process';
import {expect} from '@playwright/test';
import type {Locator, Page} from '@playwright/test';

export async function clickDropdownItem(page: Page, trigger: Locator, itemText: string) {
  await trigger.click();
  await page.getByText(itemText).click();
}

export async function login(page: Page) {
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(env.E2E_USER!);
  await page.getByLabel('Password').fill(env.E2E_PASSWORD!);
  await page.getByRole('button', {name: 'Sign In'}).click();
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeHidden();
}

export async function logout(page: Page) {
  await page.context().clearCookies();
  await page.goto('/');
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeVisible();
}
