import {expect, test} from '@playwright/test';
import {apiCreateFiles, apiCreatePR, apiCreateRepo, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

test('pull request diff prompts for a refresh when the head changes', async ({page, request}) => {
  const owner = `diff-refresh-${randomString(8)}`;
  await apiCreateUser(request, owner);
  const headers = apiUserHeaders(owner);
  const repo = 'diff-refresh';
  await apiCreateRepo(request, {name: repo, headers});
  await apiCreateFiles(request, owner, repo, [{path: 'first.txt', content: 'first change\n'}], {
    branch: 'main',
    newBranch: 'feature',
    headers,
  });
  const index = await apiCreatePR(request, owner, repo, 'feature', 'main', 'Refresh the diff', {headers});
  await loginUser(page, owner);

  const filesPath = `/${owner}/${repo}/pulls/${index}/files`;
  const socket = page.waitForEvent('websocket', {predicate: (ws) => new URL(ws.url()).pathname === `${filesPath}/ws`});
  await page.goto(filesPath);
  await socket;
  const refresh = page.getByRole('button', {name: 'Refresh diff'});
  await expect(refresh).toBeHidden();

  await page.getByRole('button', {name: 'Add line comment', exact: true}).click();
  const draft = page.locator('.diff-file-box textarea[name="content"]');
  await draft.fill('Comment in progress');

  await apiCreateFiles(request, owner, repo, [{path: 'second.txt', content: 'second change\n'}], {
    branch: 'feature',
    headers,
  });
  await expect(refresh).toBeVisible();
  await expect(draft).toHaveValue('Comment in progress');
  const newFile = page.locator('.diff-file-box[data-new-filename="second.txt"]');
  await expect(newFile).toHaveCount(0);
  await refresh.click();
  await expect(newFile).toBeVisible();
  await expect(refresh).toBeHidden();
});
