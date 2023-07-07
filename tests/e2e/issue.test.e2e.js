// @ts-check
import {test, expect} from '@playwright/test';
import {login_user, save_visual, load_logged_in_context} from './utils_e2e.js';

test.beforeAll(async ({browser}, workerInfo) => {
  await login_user(browser, workerInfo, 'user2');
});

test('Test New Issue', async ({browser}, workerInfo) => {
  const context = await load_logged_in_context(browser, workerInfo, 'user2');

  const page = await context.newPage();

  let response = await page.goto('/user2/repo2/issues');
  await expect(response?.status()).toBe(200); // Status OK

  // Click New Issue
  await page.getByRole('link', {name: 'New Issue'}).click();

  await expect(page).toHaveURL(`${workerInfo.project.use.baseURL}/user2/repo2/issues/new`);

  await page.locator('[name=title]').fill(`New Issue: ${workerInfo.title}`);
  await page.locator('[name=content]').fill(`
# Test Header

- [ ] Unchecked list item
- [ ] Second unchecked list item
- [x] Checked list item
`);

  // Switch to preview
  const previewButton = page.getByText('Preview');
  await previewButton.click();
  await expect(previewButton).toHaveClass(/(^|\W)active($|\W)/);
  await expect(page.locator('[data-tab-panel=markdown-previewer]')).toBeVisible();
  await expect(page.getByRole('heading', {name: 'Test Header'})).toBeVisible();

  // Create issue
  await page.getByRole('button', {name: 'Create Issue'}).click();
  await expect(page).toHaveURL(`${workerInfo.project.use.baseURL}/user2/repo2/issues/3`);

  await expect(page.getByRole('heading', {name: 'Test Header'})).toBeVisible();

  // Test checkboxes
  const checkboxes = page.locator('.task-list-item > [type=checkbox]');
  await expect(checkboxes).toHaveCount(3);
  await expect(checkboxes.first()).not.toBeChecked();
  const checkboxPostPromise = page.waitForResponse(`${workerInfo.project.use.baseURL}/user2/repo2/issues/3/content`);
  await checkboxes.first().click(); // Toggle checkbox
  await expect(checkboxes.first()).toBeChecked();
  expect((await checkboxPostPromise).status()).toBe(200); // Wait for successful content post response
  response = await page.reload(); // Reload page to check consistency
  await expect(checkboxes.first()).toBeChecked();

  await save_visual(page);
});
