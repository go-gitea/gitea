import {test, expect} from '@playwright/test';
import {login} from './utils.ts';

test('heatmap tooltip shows on hover', async ({page}) => {
  await login(page);
  await page.goto('/');
  await page.locator('.vch__day__square').first().hover();
  await expect(page.locator('.tippy-box[data-state="visible"]')).toBeVisible();
});
