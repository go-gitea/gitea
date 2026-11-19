import {expect, test} from '@playwright/test';
import {apiCreateFiles, apiCreatePR, apiCreateRepo, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

test('pull request diff prompts for a refresh when the head changes', async ({context, page, request}) => {
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
  await page.goto(filesPath);
  await expect(page.locator('html')).toHaveAttribute('data-user-events-connected', 'true');
  const secondTab = await context.newPage();
  await secondTab.goto(filesPath);
  await expect(secondTab.locator('html')).toHaveAttribute('data-user-events-connected', 'true');
  const refresh = page.getByRole('button', {name: 'Refresh', exact: true});
  await expect(refresh).toBeHidden();

  await page.getByRole('button', {name: 'Add line comment', exact: true}).click();
  const draft = page.getByRole('textbox', {name: 'Leave a comment', exact: true});
  await draft.fill('Comment in progress');

  await apiCreateFiles(request, owner, repo, [{path: 'second.txt', content: 'second change\n'}], {
    branch: 'feature',
    headers,
  });
  await expect(refresh).toBeVisible();
  await expect(secondTab.getByRole('button', {name: 'Refresh', exact: true})).toBeVisible();
  await secondTab.close();
  await expect(draft).toHaveValue('Comment in progress');
  const newFile = page.locator('.diff-file-box[data-new-filename="second.txt"]');
  await expect(newFile).toHaveCount(0);
  await draft.clear();
  await refresh.click();
  await expect(newFile).toBeVisible();
  await expect(refresh).toBeHidden();
});
