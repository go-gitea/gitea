import {test, expect} from '@playwright/test';
import {apiCreateFile, apiCreatePR, apiCreateRepo, apiCreateReview, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

test('pr review flow', async ({page, request}) => {
  const poster = `rv-poster-${randomString(8)}`;
  const reviewer = `rv-reviewer-${randomString(8)}`;
  await Promise.all([apiCreateUser(request, poster), apiCreateUser(request, reviewer)]);
  const posterHeaders = apiUserHeaders(poster);
  const repoName = `e2e-prreview-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName, headers: posterHeaders});
  await apiCreateFile(request, poster, repoName, 'added.txt', 'new content\n', {branch: 'main', newBranch: 'feat'});
  const prIndex = await apiCreatePR(request, poster, repoName, 'feat', 'main', 'review test', {headers: posterHeaders});

  // reviewer seeds an inline comment via API so the poster's UI reply exercises the reply-to-review path (#35994)
  await Promise.all([
    apiCreateReview(request, poster, repoName, prIndex, {
      comments: [{path: 'added.txt', body: 'inline to reply to', new_position: 1}],
      headers: apiUserHeaders(reviewer),
    }),
    loginUser(page, poster),
  ]);

  await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);

  // diff viewer renders the added file with its header and one added-line row
  const fileBox = page.locator('.diff-file-box[data-new-filename="added.txt"]');
  await expect(fileBox.locator('.diff-file-header .file-link')).toHaveText('added.txt');
  await expect(fileBox.locator('tr.add-code')).toHaveCount(1);

  // commits tab badge reflects the single PR commit, and the diff stats header counts one changed file
  const commitsTab = page.locator('.ui.pull.tabular.menu a.item', {has: page.locator('.octicon-git-commit')});
  await expect(commitsTab.locator('.label')).toHaveText('1');
  await expect(page.locator('.diff-detail-stats')).toContainText(/1 changed file/);

  // poster replies to the reviewer's inline comment
  const conversation = fileBox.locator('.conversation-holder');
  await conversation.locator('.comment-form-reply').click();
  const replyForm = conversation.locator('form');
  await replyForm.locator('textarea[name="content"]').fill('my reply body');
  await replyForm.getByRole('button', {name: 'Reply', exact: true}).click();
  await expect(conversation.locator('.comment-body')).toContainText(['inline to reply to', 'my reply body']);

  // switch to reviewer and submit an approve review
  await page.context().clearCookies();
  await loginUser(page, reviewer);
  await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);
  await page.locator('#review-box .js-btn-review').click();
  const panel = page.locator('.review-box-panel');
  await panel.locator('textarea[name="content"]').fill('LGTM');
  await panel.getByRole('button', {name: 'Approve', exact: true}).click();
  await expect(page.locator('.timeline-item .octicon-check').first()).toBeVisible();
  await expect(page.locator('.timeline-item').filter({hasText: 'LGTM'})).toBeVisible();
});
