import {test, expect} from '@playwright/test';

test('licenses.txt', async ({page}) => {
  const resp = await page.goto('/assets/licenses.txt');
  expect(resp?.status()).toBe(200);
  const content = await resp!.text();
  expect(content).toContain('@vue/');
  expect(content).toContain('code.gitea.io/');
});
