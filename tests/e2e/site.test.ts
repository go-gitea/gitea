import {test, expect} from '@playwright/test';

test('browser parses the site manifest', async ({page, browserName}) => {
  test.skip(browserName !== 'chromium', 'only CDP exposes the parsed manifest'); // eslint-disable-line playwright/no-skipped-test -- conditional skip, the reason is in the message
  await page.goto('/');
  const cdp = await page.context().newCDPSession(page);
  const {errors, manifest} = await cdp.send('Page.getAppManifest');
  expect(errors).toEqual([]);
  expect(manifest.icons).not.toHaveLength(0);
});
