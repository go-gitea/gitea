import {test, expect} from '@playwright/test';
import {assertNoJsError} from './utils.ts';

test('relative-time renders without errors', async ({page}) => {
  await page.goto('/devtest/relative-time');
  const relativeTime = page.getByTestId('relative-time-now');
  await expect(relativeTime).toHaveAttribute('data-tooltip-content', /.+/);
  await expect(relativeTime).toHaveText('now');
  await assertNoJsError(page);
});
