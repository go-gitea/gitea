import {test, expect} from '@playwright/test';
import {apiCreateFiles, apiCreatePR, apiCreateRepo, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

test('diff sidebar filtering', async ({page, request}) => {
  const user = `df-${randomString(8)}`;
  await apiCreateUser(request, user);
  const headers = apiUserHeaders(user);
  const repo = `e2e-difffilter-${randomString(8)}`;
  await apiCreateRepo(request, {name: repo, headers});

  await apiCreateFiles(request, user, repo, [
    {path: 'src/a.ts', content: 'a\n'},
    {path: 'src/b.ts', content: 'b\n'},
    {path: 'styles/x.css', content: 'x\n'},
    {path: 'docs/intro.md', content: 'r\n'},
    {path: 'Makefile', content: 'm\n'},
  ], {branch: 'main', newBranch: 'feat', headers});
  const prIndex = await apiCreatePR(request, user, repo, 'feat', 'main', 'diff filter test', {headers});

  await loginUser(page, user);
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files`);

  const tree = page.locator('#diff-file-tree');
  const items = tree.locator('.item-file');
  const search = tree.getByRole('textbox');
  const filterTrigger = tree.getByRole('button', {name: 'Filter by file extension'});

  // every PR file is listed
  await expect(items).toHaveCount(5);

  // sidebar leaves the diff column the bulk of the viewport
  const boxesWidth = (await page.locator('#diff-file-boxes').boundingBox())!.width;
  const treeWidth = (await tree.boundingBox())!.width;
  expect(boxesWidth).toBeGreaterThan(treeWidth * 2);

  // search filters tree and file boxes
  await search.fill('a.ts');
  await expect(items).toHaveText([/a\.ts/]);
  await expect(page.locator('.diff-file-box[data-new-filename="src/a.ts"]')).toBeVisible();

  await tree.getByRole('button', {name: 'Clear filter'}).click();
  await expect(items).toHaveCount(5);

  // empty-result placeholder
  await search.fill('zzz-no-such-file');
  await expect(page.locator('#diff-no-matches')).toBeVisible();
  await search.fill('');

  // extension filter: open panel, deselect .ts, only the other extensions remain
  await filterTrigger.click();
  await page.getByRole('menuitemcheckbox', {name: '.ts'}).click();
  await expect(items).toHaveCount(3);
  await expect(filterTrigger).toHaveClass(/\bindicator-dot\b/);

  // "Select none" hides everything via the empty-result placeholder
  await page.getByRole('menuitem', {name: 'Select none'}).click();
  await expect(items).toHaveCount(0);
  await expect(page.locator('#diff-no-matches')).toBeVisible();

  // "Select all" clears the filter and restores everything
  await page.getByRole('menuitem', {name: 'Select all'}).click();
  await expect(items).toHaveCount(5);
  await expect(filterTrigger).not.toHaveClass(/\bindicator-dot\b/);
});
