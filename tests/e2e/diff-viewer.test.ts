import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateFile, apiCreatePR, apiCreateRepo, login, randomString} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('diff viewer renders file box', async ({page, request}) => {
  const repoName = `e2e-diff-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName});
  await apiCreateFile(request, owner, repoName, 'added.txt', 'only on feat\n', {branch: 'main', newBranch: 'feat'});
  const [, prIndex] = await Promise.all([
    login(page),
    apiCreatePR(request, owner, repoName, 'feat', 'main', 'diff test'),
  ]);

  await page.goto(`/${owner}/${repoName}/pulls/${prIndex}/files`);
  const fileBox = page.locator('.diff-file-box[data-new-filename="added.txt"]');
  await expect(fileBox.locator('.diff-file-header .file-link')).toHaveText('added.txt');
  await expect(fileBox.locator('tr.add-code')).toHaveCount(1);
});
