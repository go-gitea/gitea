// @ts-check
import {test, expect} from '@playwright/test';

test('Load Homepage', async ({page}) => {
  const response = await page.goto('/');
  await expect(response?.status()).toBe(200); // Status OK
  await expect(page).toHaveTitle(/^Gitea: Git with a cup of tea\s*$/);
  await expect(page.locator('.logo')).toHaveAttribute('src', '/assets/img/logo.svg');
});

test('Test Register Form', async ({page}, workerInfo) => {
  const response = await page.goto('/user/sign_up');
  await expect(response?.status()).toBe(200); // Status OK
  await page.type('input[name=user_name]', `e2e-test-${workerInfo.workerIndex}`);
  await page.type('input[name=email]', `e2e-test-${workerInfo.workerIndex}@test.com`);
  await page.type('input[name=password]', 'test123');
  await page.type('input[name=retype]', 'test123');
  await page.click('form button.ui.green.button:visible');
  // Make sure we routed to the home page. Else login failed.
  await expect(page.url()).toBe(`${workerInfo.project.use.baseURL}/`);
  await expect(page.locator('.dashboard-navbar span>img.ui.avatar.image')).toBeVisible();
  await expect(page.locator('.ui.positive.message.flash-success')).toHaveText('Account was successfully created.');
  // Optionally include visual testing
  if (process.env.VISUAL_TEST) {
    await expect(page).toHaveScreenshot({ timeout: 20000, 
      mask: [
        page.locator('footer div.ui.left'), 
        page.locator('.dashboard-navbar span>img.ui.avatar.image'),
      ]
    });
  }
});
