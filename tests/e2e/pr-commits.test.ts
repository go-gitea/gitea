import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import type {APIRequestContext} from '@playwright/test';
import {apiCreateFile, apiCreatePR, apiCreateRepo, login, randomString} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

async function setupPRWithCommits(request: APIRequestContext, commitMessages: string[]) {
  const repoName = `e2e-prcommits-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName});
  // first commit creates the `feat` branch; subsequent commits must be sequential (branch-ref race)
  await apiCreateFile(request, owner, repoName, `file-0.txt`, `content 0\n`, {branch: 'main', newBranch: 'feat', message: commitMessages[0]});
  for (let index = 1; index < commitMessages.length; index++) {
    await apiCreateFile(request, owner, repoName, `file-${index}.txt`, `content ${index}\n`, {branch: 'feat', message: commitMessages[index]});
  }
  const prIndex = await apiCreatePR(request, owner, repoName, 'feat', 'main', 'commits test');
  return {repoName, prIndex};
}

test.describe('pr commits tab', () => {
  test('new commit appears', async ({page, request}) => {
    const [, {repoName, prIndex}] = await Promise.all([login(page), setupPRWithCommits(request, ['initial'])]);
    await page.goto(`/${owner}/${repoName}/pulls/${prIndex}/commits`);
    await expect(page.locator('#commits-table tbody.commit-list tr')).toHaveCount(1);

    await apiCreateFile(request, owner, repoName, 'added-later.txt', 'x\n', {branch: 'feat', message: 'appended'});
    await page.reload();

    await expect(page.locator('#commits-table tbody.commit-list .commit-summary')).toHaveText(['appended', 'initial']);
  });
});
