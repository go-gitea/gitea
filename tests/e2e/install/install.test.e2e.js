// @ts-check
import {test, expect} from '@playwright/test';

test('Load Install Page', async ({page}) => {
  const response = await page.goto('/');

  await expect(response?.status()).toBe(200); // Status OK
  await expect(page).toHaveTitle(
    /^Installation - Gitea: Git with a cup of tea\s*$/,
  );
});

test('Perform Install', async ({page}) => {
  const response = await page.goto('/');

  await expect(response?.status()).toBe(200); // Status OK
  await expect(page).toHaveTitle(
    /^Installation - Gitea: Git with a cup of tea\s*$/,
  );

  await page.getByRole('combobox').click();

  await page.getByRole('option', {name: 'SQLite3'}).click();

  // Past this point, testing the install page functionality runs into a few issues:
  //  - The existing fixtures/setup don't handle various db stuff that install
  //      performs (clobbers subsequent tests)
  //  - Existing .ini and db files trigger the "reinstall" dialogue rather than normal install
  //  - Successfully reinstalling seems to work, but the backend will drop the "install" routes
  //      and is not setup to start the "normal" routes (thats something the CLI handles)
  // Those are probably all resolvable but might take a larger re-work of the fixtures/setup logic

  // await page.getByRole('button', {name: "Install Gitea"}).click();

  // await expect(page.getByTestId('reinstall_confirm_first'), 'performs database checks').toBeVisible();

  // await page.waitForLoadState('domcontentloaded');

  // await page.getByTestId('reinstall_confirm_first').check();
  // await page.getByTestId('reinstall_confirm_second').check();
  // await page.getByTestId('reinstall_confirm_third').check();

  // await page.getByRole('button', {name: "Install Gitea"}).click();

  // await page.getByRole('link', { name: 'Loading…' });
});
