import {test, expect} from '@playwright/test';
import {apiCreateFile, apiUpdateFile, apiCreateBranch, apiCreatePR, apiCreateRepo, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

test('expand and collapse all hidden lines of a diff file', async ({page, request}) => {
  const poster = `diffexp-${randomString(8)}`;
  await apiCreateUser(request, poster);
  const headers = apiUserHeaders(poster);
  const repoName = `e2e-diffexpandall-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName, headers});

  // a file with only its first and last lines changed leaves a large untouched block in the middle,
  // which the diff viewer hides behind a single "gap" row with an expander button
  const lines = Array.from({length: 40}, (_, i) => `line${String(i + 1).padStart(2, '0')}`);
  await apiCreateFile(request, poster, repoName, 'big.txt', `${lines.join('\n')}\n`, {branch: 'main'});
  await apiCreateBranch(request, poster, repoName, 'feat');
  const changedLines = [...lines];
  changedLines[0] = 'line01-updated';
  changedLines[39] = 'line40-updated';
  await apiUpdateFile(request, poster, repoName, 'big.txt', `${changedLines.join('\n')}\n`, {branch: 'feat'});
  const prIndex = await apiCreatePR(request, poster, repoName, 'feat', 'main', 'expand all lines test', {headers});

  await loginUser(page, poster);
  await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);

  const fileBox = page.locator('.diff-file-box[data-new-filename="big.txt"]');
  const originalRowCount = await fileBox.locator('table.chroma tr').count();

  // the hidden middle line is not part of the initial render
  await expect(fileBox.getByText('line20', {exact: true})).not.toBeAttached();

  const expandButton = fileBox.getByRole('button', {name: 'Expand all lines'});
  await expect(expandButton).toBeVisible();
  await expandButton.click();

  // once expanded, the gap and its expander button are gone, and the previously hidden line is visible
  await expect(fileBox.getByText('line20', {exact: true})).toBeVisible();
  await expect(fileBox.locator('.code-expander-buttons[data-expand-all-url]')).toHaveCount(0);

  // clicking again collapses the file back to its original, pristine rendering
  const collapseButton = fileBox.getByRole('button', {name: 'Collapse expanded lines'});
  await collapseButton.click();
  await expect(fileBox.getByText('line20', {exact: true})).not.toBeAttached();
  await expect(fileBox.locator('table.chroma tr')).toHaveCount(originalRowCount);

  // folding hides the file body via CSS (as happens for vendored/generated files, or files already marked
  // "viewed" in a PR); "expand all lines" must unfold it too, otherwise the newly expanded content would
  // stay invisible behind the fold (see go-gitea/gitea#36663)
  await fileBox.locator('.fold-file').click();
  await expect(fileBox.locator('.diff-file-body')).toBeHidden();
  await expandButton.click();
  await expect(fileBox).not.toHaveAttribute('data-folded', 'true');
  await expect(fileBox.getByText('line20', {exact: true})).toBeVisible();
});
