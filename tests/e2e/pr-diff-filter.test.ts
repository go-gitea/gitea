import {test, expect} from '@playwright/test';
import {apiChangeFiles, apiCreateFiles, apiCreatePR, apiCreateRepo, apiCreateReview, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

test('diff sidebar filtering', async ({page, request}) => {
  const user = `df-${randomString(8)}`;
  await apiCreateUser(request, user);
  const headers = apiUserHeaders(user);
  const repo = `e2e-difffilter-${randomString(8)}`;
  await apiCreateRepo(request, {name: repo, headers});

  await apiCreateFiles(request, user, repo, [
    {path: 'src/a.ts', content: 'a\n'},
    {path: 'old.md', content: 'r\n'},
    {path: 'gone.txt', content: 'g\n'},
  ], {branch: 'main', headers});
  await apiChangeFiles(request, user, repo, [
    {operation: 'update', path: 'src/a.ts', content: 'a2\n'},
    {operation: 'update', path: 'docs/intro.md', fromPath: 'old.md', content: 'r\n'},
    {operation: 'delete', path: 'gone.txt'},
    {operation: 'create', path: 'src/b.ts', content: 'b\n'},
    {operation: 'create', path: 'src/.eslintrc', content: 'e\n'},
    {operation: 'create', path: 'styles/x.css', content: 'x\n'},
    {operation: 'create', path: 'Makefile', content: 'm\n'},
  ], {branch: 'main', newBranch: 'feat', headers});
  const prIndex = await apiCreatePR(request, user, repo, 'feat', 'main', 'diff filter test', {headers});
  await apiCreateReview(request, user, repo, prIndex, {comments: [{path: 'src/a.ts', body: 'filter-comment', new_position: 1}], headers});

  await loginUser(page, user);
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files`);

  const tree = page.locator('#diff-file-tree');
  const items = tree.locator('.item-file');
  const search = tree.getByRole('textbox');
  const filterTrigger = tree.getByRole('button', {name: 'Filter options'});
  const fileBox = (name: string) => page.locator(`.diff-file-box[data-new-filename="${name}"]`);

  // every PR file is listed
  await expect(items).toHaveCount(7);
  await expect(fileBox('src/a.ts')).toBeVisible();

  // sidebar leaves the diff column the bulk of the viewport
  const boxesWidth = (await page.locator('#diff-viewer').boundingBox())!.width;
  const treeWidth = (await tree.boundingBox())!.width;
  expect(boxesWidth).toBeGreaterThan(treeWidth * 2);

  // search filters tree and file boxes
  await search.fill('a.ts');
  await expect(items).toHaveText([/a\.ts/]);
  await expect(fileBox('styles/x.css')).toHaveCount(0);

  await tree.getByRole('button', {name: 'Clear filter'}).click();
  await expect(items).toHaveCount(7);

  // empty-result placeholder
  await search.fill('zzz-no-such-file');
  await expect(page.locator('#diff-no-matches')).toBeVisible();
  await search.fill('');

  // statuses, then extensions sorted alphabetically, then dotfiles, then files without extension
  await filterTrigger.click();
  const menuItems = page.getByRole('menuitemcheckbox');
  await expect(menuItems).toHaveText(['Added4', 'Modified1', 'Deleted1', 'Renamed1', 'Has comments1', '.css1', '.md1', '.ts2', '.txt1', 'Dotfiles1', 'No extension1', 'All extensions']);

  // the status filter applies to the tree and the viewer
  await page.getByRole('menuitemcheckbox', {name: 'Added'}).click();
  await expect(items).toHaveCount(3);
  await expect(fileBox('styles/x.css')).toHaveCount(0);
  await expect(filterTrigger).toHaveClass(/\bindicator-dot\b/);
  await page.getByRole('menuitemcheckbox', {name: 'Has comments'}).click();
  await expect(items).toHaveText([/a\.ts/]);
  await page.getByRole('menuitemcheckbox', {name: 'Has comments'}).click();
  await page.getByRole('menuitemcheckbox', {name: 'Added'}).click();
  await expect(items).toHaveCount(7);

  // deselecting .ts leaves the other extensions
  const allExtensions = page.getByRole('menuitemcheckbox', {name: 'All extensions'});
  await page.getByRole('menuitemcheckbox', {name: '.ts'}).click();
  await expect(items).toHaveCount(5);
  await expect(allExtensions).toHaveAttribute('aria-checked', 'false');

  // "All extensions" cycles through select all, select none and back
  await allExtensions.click();
  await expect(allExtensions).toHaveAttribute('aria-checked', 'true');
  await expect(items).toHaveCount(7);
  await allExtensions.click();
  await expect(items).toHaveCount(0);
  await allExtensions.click();
  await expect(items).toHaveCount(7);
  await expect(filterTrigger).not.toHaveClass(/\bindicator-dot\b/);

  // the extension filter lives in the URL and survives a reload
  await page.getByRole('menuitemcheckbox', {name: '.ts'}).click();
  await expect(page).toHaveURL(/file-filters/);
  await page.reload();
  await expect(items).toHaveCount(5);
  await expect(filterTrigger).toHaveClass(/\bindicator-dot\b/);

  // the comments tab lists the threads of the shown files
  await tree.getByRole('tab', {name: /Comments/}).click();
  await expect(tree.getByText('No comments')).toBeVisible();
  await filterTrigger.click();
  await page.getByRole('menuitemcheckbox', {name: '.ts'}).click();
  await page.keyboard.press('Escape');
  const thread = tree.getByRole('link').filter({hasText: 'filter-comment'});
  await expect(thread).toHaveAttribute('href', /^#issuecomment-\d+$/);
  await thread.click();
  await expect(page).toHaveURL(/#issuecomment-\d+$/);
  await expect(page.locator('.diff-conversation .comment-body .markup')).toBeInViewport();
  await tree.getByRole('tab', {name: 'Files'}).click();
  await expect(items).toHaveCount(7);

  // hiding the file tree drops the filters
  await filterTrigger.click();
  await page.getByRole('menuitemcheckbox', {name: 'Modified'}).click();
  await expect(fileBox('src/a.ts')).toHaveCount(0);
  await page.locator('.diff-toggle-file-tree-button').click();
  await expect(tree.locator('.diff-file-tree-wrapper')).toBeHidden();
  await expect(fileBox('src/a.ts')).toBeVisible();
});
