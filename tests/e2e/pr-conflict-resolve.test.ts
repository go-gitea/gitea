// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {expect, test} from '@playwright/test';
import {apiCreateBranch, apiCreateFile, apiCreatePR, apiCreateUser, apiUserHeaders, loginUser, randomString, timeoutFactor} from './utils.ts';

/**
 * Creates a PR where `feat` and `main` both modify the same line in `conflict.txt`,
 * so Gitea's merge-check will mark it as conflicted.
 */
async function setupConflictPR(request: Parameters<typeof apiCreateFile>[0], owner: string, repo: string) {
  const headers = apiUserHeaders(owner);

  // Create conflict.txt on main with initial content.
  await apiCreateFile(request, owner, repo, 'conflict.txt', 'ancestor line\n', {branch: 'main', message: 'add conflict.txt'});

  // Branch off for the feature branch and make a diverging change.
  await apiCreateBranch(request, owner, repo, 'feat');
  await apiCreateFile(request, owner, repo, 'conflict.txt', 'feature change\n', {branch: 'feat', message: 'feat change'});

  // Make a conflicting change on main AFTER the branch was created.
  await apiCreateFile(request, owner, repo, 'conflict.txt', 'main change\n', {branch: 'main', message: 'main change'});

  // Create the PR.
  const prIndex = await apiCreatePR(request, owner, repo, 'feat', 'main', 'conflict pr', {headers});
  return prIndex;
}

test('pr conflict resolve — file list and editor', async ({page, request}) => {
  const owner = `conflict-${randomString(6)}`;
  const repoName = `e2e-conflict-${randomString(6)}`;
  await apiCreateUser(request, owner);
  const headers = apiUserHeaders(owner);
  await request.post(`/api/v1/user/repos`, {
    headers,
    data: {name: repoName, auto_init: true, default_branch: 'main'},
  });

  const prIndex = await setupConflictPR(request, owner, repoName);

  await loginUser(page, owner);

  // Visit the PR and wait for the conflict check to complete.
  await page.goto(`/${owner}/${repoName}/pulls/${prIndex}`);

  // Reload the merge-box every second until it leaves the "checking" state.
  const mergeBox = page.locator('.pull-merge-box');
  await mergeBox.waitFor({state: 'visible', timeout: 15_000 * timeoutFactor});

  // Wait until the "Resolve conflicts" link appears (means the check finished
  // and found at least one conflicting file).
  const resolveLink = page.getByRole('link', {name: 'Resolve conflicts'});
  await expect(resolveLink).toBeVisible({timeout: 20_000 * timeoutFactor});

  // ── Conflict resolver page ─────────────────────────────────────────────
  await resolveLink.click();
  await page.waitForURL(new RegExp(`/${owner}/${repoName}/pulls/${prIndex}/conflicts/editor/`));

  // Sidebar lists the conflicting file.
  const sidebar = page.locator('.conflict-sidebar');
  await expect(sidebar).toBeVisible();
  const fileItem = sidebar.locator('.conflict-file-item');
  await expect(fileItem).toHaveCount(1); // only conflict.txt
  await expect(fileItem.first()).toContainText('conflict.txt');

  // Editor area loads and shows the current file path.
  const editorArea = page.locator('.conflict-editor-area');
  await expect(editorArea).toBeVisible();
  const filePath = editorArea.locator('.conflict-current-file-path');
  await expect(filePath).toHaveText('conflict.txt');

  // CodeMirror editor is visible and contains conflict markers.
  const cmEditor = page.locator('#conflict-editor-root .cm-content');
  await expect(cmEditor).toBeVisible({timeout: 10_000 * timeoutFactor});
  const editorText = await cmEditor.textContent();
  expect(editorText).toContain('<<<<<<<');
  expect(editorText).toContain('>>>>>>>');

  // ── Resolve the conflict ───────────────────────────────────────────────
  // Clear the editor content and type a clean resolution.
  await cmEditor.click();
  await page.keyboard.press('Control+a');
  await page.keyboard.type('resolved line\n');

  // "Mark as resolved" button should be enabled and clickable.
  const markBtn = editorArea.getByRole('button', {name: /mark as resolved/i});
  await expect(markBtn).toBeVisible();
  await markBtn.click();

  // After marking, the sidebar icon for the file changes to a checkmark.
  await expect(fileItem.first().locator('.conflict-file-status-icon')).toHaveText('✓');

  // "Commit merge" button should now be enabled.
  const commitBtn = sidebar.locator('.conflict-commit-btn');
  await expect(commitBtn).toBeEnabled({timeout: 5_000 * timeoutFactor});

  // ── Commit the resolution ──────────────────────────────────────────────
  await commitBtn.click();

  // After commit, the browser should redirect back to the PR page.
  await page.waitForURL(new RegExp(`/${owner}/${repoName}/pulls/${prIndex}$`));

  // "Resolve conflicts" link must be gone after successful commit.
  await expect(page.getByRole('link', {name: 'Resolve conflicts'})).toHaveCount(0, {timeout: 20_000 * timeoutFactor});
});

test('pr conflict resolve — file switching saves content', async ({page, request}) => {
  const owner = `conflict2-${randomString(6)}`;
  const repo = `e2e-conflict2-${randomString(6)}`;
  await apiCreateUser(request, owner);
  const headers = apiUserHeaders(owner);
  await request.post(`/api/v1/user/repos`, {
    headers,
    data: {name: repo, auto_init: true, default_branch: 'main'},
  });

  // Create TWO conflicting files so we can test switching.
  await apiCreateFile(request, owner, repo, 'file-a.txt', 'ancestor a\n', {branch: 'main', message: 'add files'});
  await apiCreateFile(request, owner, repo, 'file-b.txt', 'ancestor b\n', {branch: 'main', message: 'add file-b'});
  await apiCreateBranch(request, owner, repo, 'feat');
  await apiCreateFile(request, owner, repo, 'file-a.txt', 'feat change a\n', {branch: 'feat', message: 'feat a'});
  await apiCreateFile(request, owner, repo, 'file-b.txt', 'feat change b\n', {branch: 'feat', message: 'feat b'});
  await apiCreateFile(request, owner, repo, 'file-a.txt', 'main change a\n', {branch: 'main', message: 'main a'});
  await apiCreateFile(request, owner, repo, 'file-b.txt', 'main change b\n', {branch: 'main', message: 'main b'});
  const prIndex = await apiCreatePR(request, owner, repo, 'feat', 'main', 'two-file conflict', {headers});

  await loginUser(page, owner);
  await page.goto(`/${owner}/${repo}/pulls/${prIndex}`);
  const resolveLink = page.getByRole('link', {name: 'Resolve conflicts'});
  await expect(resolveLink).toBeVisible({timeout: 20_000 * timeoutFactor});
  await resolveLink.click();

  // Both files should appear in the sidebar.
  const sidebar = page.locator('.conflict-sidebar');
  const fileItems = sidebar.locator('.conflict-file-item');
  await expect(fileItems).toHaveCount(2, {timeout: 10_000 * timeoutFactor});

  // Type something in the first file, then switch to the second file.
  const cmEditor = page.locator('#conflict-editor-root .cm-content');
  await expect(cmEditor).toBeVisible({timeout: 10_000 * timeoutFactor});
  await cmEditor.click();
  await page.keyboard.press('Control+a');
  await page.keyboard.type('resolution for first file\n');

  // Click the second file in the sidebar.
  const secondFileItem = fileItems.nth(1);
  await secondFileItem.click();

  // Editor should switch to the second file (path in header changes).
  const firstFileName = (await fileItems.nth(0).locator('span.tw-truncate').textContent()) ?? '';
  const filePath = page.locator('.conflict-current-file-path');
  await expect(filePath).not.toHaveText(firstFileName);

  // Switch back to the first file — the content we typed should be preserved.
  await fileItems.nth(0).click();
  const firstFileContent = await cmEditor.textContent();
  expect(firstFileContent).toContain('resolution for first file');
});
