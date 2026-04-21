import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateFile, apiCreatePR, apiCreateRepo, login, randomString} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('new commit appears on pr commits tab', async ({page, request}) => {
  const repoName = `e2e-prcommits-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName});
  await apiCreateFile(request, owner, repoName, 'file-0.txt', 'content 0\n', {branch: 'main', newBranch: 'feat', message: 'initial'});
  const [, prIndex] = await Promise.all([
    login(page),
    apiCreatePR(request, owner, repoName, 'feat', 'main', 'commits test'),
  ]);

  await page.goto(`/${owner}/${repoName}/pulls/${prIndex}/commits`);
  await expect(page.locator('#commits-table tbody.commit-list tr')).toHaveCount(1);

  await apiCreateFile(request, owner, repoName, 'added-later.txt', 'x\n', {branch: 'feat', message: 'appended'});
  await page.reload();

  await expect(page.locator('#commits-table tbody.commit-list .commit-summary')).toHaveText(['appended', 'initial']);
});
