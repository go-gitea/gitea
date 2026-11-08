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
