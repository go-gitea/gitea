// @ts-check
import {test, expect} from '@playwright/test';

test('Load Install Page', async ({page}) => {
  const response = await page.goto('/');

  await expect(response?.status()).toBe(200); // Status OK
  await expect(page).toHaveTitle(
    /^Installation - Gitea: Git with a cup of tea\s*$/,
  );
});
