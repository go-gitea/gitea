import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo, randomString} from './utils.ts';

test('codeeditor textarea updates correctly', async ({page, request}) => {
  const repoName = `e2e-codeeditor-${randomString(8)}`;
  await Promise.all([apiCreateRepo(request, {name: repoName}), login(page)]);
  try {
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/_new/main`);
    await page.getByPlaceholder('Name your file…').fill('test.js');
    await expect(page.locator('[data-tab="write"] .editor-loading')).toBeHidden();
    const editor = page.locator('.cm-content[role="textbox"]');
    await expect(editor).toBeVisible();
    await editor.click();
    await page.keyboard.type('const hello = "world";');
    await expect(page.locator('textarea[name="content"]')).toHaveValue('const hello = "world";');
  } finally {
    await apiDeleteRepo(request, env.GITEA_TEST_E2E_USER, repoName);
  }
});
