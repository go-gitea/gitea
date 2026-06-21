import {test, expect} from '@playwright/test';
import {login} from './utils.ts';

test('heatmap tooltip shows on hover', async ({page}) => {
  await login(page);
  await page.goto('/');
  await page.locator('.heatmap-day').first().hover();
  await expect(page.locator('.tippy-box[data-state="visible"]')).toBeVisible();
});
