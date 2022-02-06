// @ts-check
import {test, expect} from '@playwright/test';

test('Load Homepage', async ({page}) => {
  const response = await page.goto('/');
  await expect(response.status()).toBe(200); // Status OK
  await expect(page).toHaveTitle(/^Gitea: Test\s*$/);
  await expect(page.locator('.logo')).toHaveAttribute('src', '/assets/img/logo.svg');
});
