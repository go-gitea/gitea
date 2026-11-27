import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login} from './utils.ts';

test('heatmap tooltip shows on hover', async ({page}) => {
  await login(page);
  await page.goto('/');
  await page.locator('.heatmap-day').first().hover();
  await expect(page.locator('.tippy-box[data-state="visible"]')).toBeVisible();
});

test('heatmap year selector filters activity from the navigation', async ({page}) => {
  await login(page);
  await page.setViewportSize({width: 390, height: 844});

  const year = String(new Date().getFullYear());
  for (const path of ['/', `/${env.GITEA_TEST_E2E_USER}?tab=activity`]) {
    await page.goto(path);
    const selector = page.locator('#heatmap-year-nav').getByRole('combobox', {name: 'Time range'});
    await expect(selector).toBeInViewport();
    await expect(page.locator('#user-heatmap select')).toHaveCount(0);
    await selector.selectOption(year);
    await expect(page).toHaveURL(new RegExp(`[?&]year=${year}(?:&|$)`));
    await expect(selector).toHaveValue(year);
  }
});
