import {expect} from '@playwright/test';

const ARTIFACTS_PATH = `tests/e2e/test-artifacts`;
const LOGIN_PASSWORD = 'password';

// log in user and store session info. This should generally be
//  run in test.beforeAll(), then the session can be loaded in tests.
export async function login_user(browser, workerInfo, user) {
  // Set up a new context
  const context = await browser.newContext();
  const page = await context.newPage();

  // Route to login page
  // Note: this could probably be done more quickly with a POST
  const response = await page.goto('/user/login');
  await expect(response?.status()).toBe(200); // Status OK

  // Fill out form
  await page.type('input[name=user_name]', user);
  await page.type('input[name=password]', LOGIN_PASSWORD);
  await page.click('form button.ui.primary.button:visible');

  await page.waitForLoadState('networkidle');

  await expect(page.url(), {message: `Failed to login user ${user}`}).toBe(`${workerInfo.project.use.baseURL}/`);

  // Save state
  await context.storageState({path: `${ARTIFACTS_PATH}/state-${user}-${workerInfo.workerIndex}.json`});

  return context;
}

export async function load_logged_in_context(browser, workerInfo, user) {
  let context;
  try {
    context = await browser.newContext({storageState: `${ARTIFACTS_PATH}/state-${user}-${workerInfo.workerIndex}.json`});
  } catch (err) {
    if (err.code === 'ENOENT') {
      throw new Error(`Could not find state for '${user}'. Did you call login_user(browser, workerInfo, '${user}') in test.beforeAll()?`);
    }
  }
  return context;
}

export async function save_visual(page) {
  // Optionally include visual testing
  if (process.env.VISUAL_TEST) {
    await page.waitForLoadState('networkidle');
    // Mock page/version string
    await page.locator('footer div.ui.left').evaluate((node) => node.innerHTML = 'MOCK');
    await expect(page).toHaveScreenshot({
      fullPage: true,
      timeout: 20000,
      mask: [
        page.locator('.dashboard-navbar span>img.ui.avatar'),
        page.locator('.ui.dropdown.jump.item span>img.ui.avatar'),
      ],
    });
  }
}
