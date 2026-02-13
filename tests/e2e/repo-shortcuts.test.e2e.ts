import {test, expect} from '@playwright/test';
import {login_user, load_logged_in_context} from './utils_e2e.ts';

test.beforeAll(async ({browser}, workerInfo) => {
  await login_user(browser, workerInfo, 'user2');
});

test.describe('Repository Keyboard Shortcuts', () => {
  test('T key focuses file search input', async ({browser}, workerInfo) => {
    const context = await load_logged_in_context(browser, workerInfo, 'user2');
    const page = await context.newPage();

    // Navigate to a repository page with file listing
    await page.goto('/user2/repo1');

    // Verify the file search input exists and has the keyboard hint
    const fileSearchInput = page.getByPlaceholder('Go to file');
    await expect(fileSearchInput).toBeVisible();

    // Verify the keyboard hint is visible
    const kbdHint = page.locator('.repo-file-search-input-wrapper kbd');
    await expect(kbdHint).toBeVisible();
    await expect(kbdHint).toHaveText('T');

    // Press T key to focus the file search input
    await page.keyboard.press('t');

    // Verify the input is focused
    await expect(fileSearchInput).toBeFocused();
  });

  test('T key does not trigger when typing in input', async ({browser}, workerInfo) => {
    const context = await load_logged_in_context(browser, workerInfo, 'user2');
    const page = await context.newPage();

    // Navigate to a repository page
    await page.goto('/user2/repo1');

    // Focus on file search first
    const fileSearchInput = page.getByPlaceholder('Go to file');
    await fileSearchInput.click();

    // Type something including 't'
    await page.keyboard.type('test');

    // Verify the input still has focus and contains the typed text
    await expect(fileSearchInput).toBeFocused();
    await expect(fileSearchInput).toHaveValue('test');
  });

  test('S key focuses code search input on repo home', async ({browser}, workerInfo) => {
    const context = await load_logged_in_context(browser, workerInfo, 'user2');
    const page = await context.newPage();

    // Navigate to repo home page where code search is available
    await page.goto('/user2/repo1');

    // The code search input is in the sidebar
    const codeSearchInput = page.getByPlaceholder('Search code…');
    await expect(codeSearchInput).toBeVisible();

    // Verify the keyboard hint is visible
    const kbdHint = page.locator('.repo-code-search-input-wrapper .repo-search-shortcut-hint');
    await expect(kbdHint).toBeVisible();
    await expect(kbdHint).toHaveText('S');

    // Press S key to focus the code search input
    await page.keyboard.press('s');

    // Verify the input is focused
    await expect(codeSearchInput).toBeFocused();
  });

  test('File search keyboard hint hides when input has value', async ({browser}, workerInfo) => {
    const context = await load_logged_in_context(browser, workerInfo, 'user2');
    const page = await context.newPage();

    // Navigate to a repository page
    await page.goto('/user2/repo1');

    // Check file search kbd hint
    const fileSearchInput = page.getByPlaceholder('Go to file');
    const fileKbdHint = page.locator('.repo-file-search-input-wrapper kbd');

    // Initially the hint should be visible
    await expect(fileKbdHint).toBeVisible();

    // Focus and type in the file search
    await fileSearchInput.click();
    await page.keyboard.type('test');

    // The hint should now be hidden (Vue component handles this with v-show)
    await expect(fileKbdHint).toBeHidden();
  });

  test('Code search keyboard hint hides when input has value', async ({browser}, workerInfo) => {
    const context = await load_logged_in_context(browser, workerInfo, 'user2');
    const page = await context.newPage();

    // Navigate to a repository page
    await page.goto('/user2/repo1');

    const codeSearchInput = page.getByPlaceholder('Search code…');
    await expect(codeSearchInput).toBeVisible();

    const codeKbdHint = page.locator('.repo-code-search-input-wrapper .repo-search-shortcut-hint');

    // Initially the hint should be visible
    await expect(codeKbdHint).toBeVisible();

    // Focus and type in the code search
    await codeSearchInput.click();
    await page.keyboard.type('search');

    // The hint should now be hidden
    await expect(codeKbdHint).toBeHidden();
  });

  test('Shortcuts do not trigger with modifier keys', async ({browser}, workerInfo) => {
    const context = await load_logged_in_context(browser, workerInfo, 'user2');
    const page = await context.newPage();

    // Navigate to a repository page
    await page.goto('/user2/repo1');

    const fileSearchInput = page.getByPlaceholder('Go to file');

    // Click somewhere else first to ensure nothing is focused
    await page.locator('body').click();

    // Press Ctrl+T (should not focus file search - this is typically "new tab" in browsers)
    await page.keyboard.press('Control+t');

    // The file search input should NOT be focused
    await expect(fileSearchInput).not.toBeFocused();
  });
});
