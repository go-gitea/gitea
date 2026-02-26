import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo} from './utils.ts';

test.describe('codeeditor', () => {
  const repoName = `e2e-codeeditor-${Date.now()}`;

  test.beforeAll(async ({request}) => {
    await apiCreateRepo(request, {name: repoName});
  });

  test.afterAll(async ({request}) => {
    await apiDeleteRepo(request, env.GITEA_TEST_E2E_USER, repoName);
  });

  test('textarea updates correctly', async ({page}) => {
    await login(page);
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/_new/main`);
    await page.getByPlaceholder('Name your file…').fill('test.js');
    await expect(page.locator('.editor-loading')).toBeHidden();
    const editor = page.locator('.cm-content[role="textbox"]');
    await expect(editor).toBeVisible();
    await editor.click();
    await page.keyboard.type('const hello = "world";');
    await expect(page.locator('textarea[name="content"]')).toHaveValue('const hello = "world";');
  });
});
