import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateFiles, apiCreateRepo, randomString} from './utils.ts';

test('codeeditor textarea updates correctly', async ({page, request}) => {
  const repoName = `e2e-codeeditor-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName});
      await apiCreateFiles(request, owner, repoName, [{path: '.editorconfig', content: '[*]\nindent_size = 2\n'}], {newBranch: 'feature'});
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/_new/feature`);
  await page.locator('.cm-content[role="textbox"]').click();
  await page.keyboard.press('Tab');
  await expect(page.locator('textarea[name="content"]')).toHaveValue('  ');
});

test('codeeditor diff preview', async ({page, request}) => {
  const repoName = `e2e-codeeditor-${randomString(8)}`;
  await Promise.all([apiCreateRepo(request, {name: repoName}), login(page)]);
  await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/_edit/main/README.md`);
  await expect(page.locator('[data-tab="write"] .editor-loading')).toBeHidden();
  await page.locator('.cm-content[role="textbox"]').click();
  await page.keyboard.type('preview line ');
  await page.locator('a[data-tab="diff"]').click();
  const diff = page.locator('.tab[data-tab="diff"]');
  await expect(diff.locator('tr.add-code')).toHaveText(/preview line/);
  await expect(diff.locator('tr.same-code')).toHaveCount(2);
});
