import {test} from '@playwright/test';
import {assertNoJsError} from './utils.ts';

test('homepage renders without errors', async ({page}) => {
  await page.goto('/');
  await page.waitForLoadState('networkidle'); // eslint-disable-line playwright/no-networkidle
  await assertNoJsError(page);
});
