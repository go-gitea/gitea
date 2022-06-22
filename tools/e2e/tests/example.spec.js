// @ts-check
import { test, expect } from '@playwright/test';

test('basic test', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/.*Gitea*/);
  //await expect(page.locator('.logo')).toHaveAttribute('src', '/assets/img/logo.svg')
});
