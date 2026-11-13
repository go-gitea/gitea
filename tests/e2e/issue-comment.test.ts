import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateIssue, apiEditIssue, randomString} from './utils.ts';

test('edit an issue, quote-reply it with an uploaded image and close it', async ({page, request}) => {
  const repoName = `e2e-issue-comment-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      await apiCreateIssue(request, {owner, repo: repoName, title: 'Comment test', body: 'Original body'});
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/issues/1`);
  const commentButton = page.getByRole('button', {name: 'Comment', exact: true});
  await expect(commentButton).toBeDisabled();

  const contextMenu = page.getByRole('heading', {level: 3}).getByRole('button', {name: 'More items'});
  await contextMenu.click();
  await page.getByRole('menuitem', {name: 'Edit', exact: true}).click();
  const issueBody = page.getByRole('article');
  await expect(issueBody.getByRole('textbox')).toHaveValue('Original body');
  await issueBody.getByRole('textbox').fill('Updated **body**');
  await issueBody.getByRole('button', {name: 'Save', exact: true}).click();
  await expect(issueBody.getByText('Updated body', {exact: true})).toBeVisible();

  await contextMenu.click();
  await page.getByRole('menuitem', {name: 'Quote reply', exact: true}).click();
  await expect(commentButton).toBeEnabled();
  const editor = page.getByRole('textbox', {name: 'Leave a comment'});
  await editor.evaluate((textarea) => {
    const dataTransfer = new DataTransfer();
    dataTransfer.items.add(new File([
      Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII='), (character) => character.charCodeAt(0)),
    ], 'pixel.png', {type: 'image/png'}));
    textarea.dispatchEvent(new DragEvent('drop', {bubbles: true, cancelable: true, dataTransfer}));
  });
  await expect(editor).toHaveValue(/^> Updated \*\*body\*\*\n\n!\[pixel\.png\]\(attachments\/[\da-f-]+\)$/);
  await commentButton.click();
  await page.waitForURL(/#issuecomment-\d+$/);
  await expect(page.getByRole('article').getByRole('blockquote')).toHaveText('Updated body');
  await expect(page.getByRole('article').getByRole('img', {name: 'pixel.png', exact: true})).toHaveJSProperty('naturalWidth', 1);

  await expect(commentButton).toBeDisabled();
  await page.getByRole('button', {name: 'Close Issue'}).click();
  await expect(page.getByRole('button', {name: 'Reopen Issue'})).toBeVisible();
});

test('emoji autocompletion in issue description', async ({page, request}) => {
  const repoName = `e2e-emoji-${randomString(8)}`;
  await Promise.all([apiCreateRepo(request, {name: repoName, autoInit: false}), login(page)]);
  await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/new`);
  const textarea = page.getByPlaceholder('Leave a comment');
  await textarea.focus();
  await textarea.pressSequentially(':tada');
  await expect(page.getByRole('option', {name: '🎉 tada'})).toBeVisible();
  await textarea.press('Tab');
  await expect(textarea).toHaveValue('🎉');
});

test('unsaved issue description prompts before leaving', async ({page, request}) => {
  const repoName = `e2e-are-you-sure-${randomString(8)}`;
  await Promise.all([apiCreateRepo(request, {name: repoName, autoInit: false}), login(page)]);
  await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/new`);
  await page.getByPlaceholder('Leave a comment').press('a');
  const dialogPromise = page.waitForEvent('dialog');
  page.once('dialog', (dialog) => dialog.dismiss());
  await page.getByRole('link', {name: 'Dashboard'}).click();
  expect((await dialogPromise).type()).toBe('beforeunload');
  await expect(page).toHaveURL(/\/issues\/new$/);
});

test('delete an edit history entry via a confirm over the history modal', async ({page, request}) => {
  const repoName = `e2e-issue-history-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      await apiCreateIssue(request, {owner, repo: repoName, title: 'History test', body: 'first'});
      await apiEditIssue(request, owner, repoName, 1, {body: 'second'});
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/issues/1`);
  await page.getByRole('button', {name: '• edited'}).click();
  await page.getByRole('menuitem').last().click();
  const history = page.getByRole('dialog');
  await history.getByRole('button', {name: 'Options'}).click();
  await page.getByRole('menuitem', {name: 'Delete from history'}).click();
  await page.getByRole('dialog', {name: 'Delete from history?'}).getByRole('button', {name: 'Confirm'}).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('button', {name: '• edited'}).click();
  await expect(page.getByRole('menuitem').filter({hasText: 'deleted'})).toHaveAttribute('aria-disabled', 'true');
});

test('reference a comment in a new issue of another repository', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repoName = `e2e-issue-ref-${randomString(8)}`;
  const targetName = `e2e-issue-ref-target-${randomString(8)}`;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      await apiCreateIssue(request, {owner, repo: repoName, title: 'Source issue', body: 'referenced text'});
    })(),
    apiCreateRepo(request, {name: targetName, autoInit: false}),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/issues/1`);
  await page.getByRole('heading', {level: 3}).getByRole('button', {name: 'More items'}).click();
  await page.getByRole('menuitem', {name: 'Reference in new issue'}).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('combobox', {name: /^Repository/}).fill(targetName);
  const target = page.getByRole('option');
  await expect(target).toHaveText([`${owner}/${targetName}`]);
  await target.click();
  await dialog.locator('input[name="title"]').fill('Referenced issue');
  await dialog.getByRole('button', {name: 'Create Issue'}).click();
  await page.waitForURL(new RegExp(`/${owner}/${targetName}/issues/1$`));
  await expect(page.getByRole('emphasis')).toContainText(`Originally posted by @${owner} in ${owner}/${repoName}#1`);
});
