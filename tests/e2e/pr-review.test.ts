import {test, expect, type APIRequestContext} from '@playwright/test';
import {
  apiAddCollaborator,
  apiCreateFiles,
  apiCreatePR,
  apiCreateRepo,
  apiCreateReview,
  apiCreateUser,
  apiUserHeaders,
  loginUser,
  logout,
  randomString,
} from './utils.ts';

async function createPullRequest(request: APIRequestContext, poster: string) {
  const posterHeaders = apiUserHeaders(poster);
  const repoName = `e2e-prreview-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName, headers: posterHeaders});
  await apiCreateFiles(request, poster, repoName, [{path: 'added.txt', content: 'new content\nsecond line\n'}], {branch: 'main', newBranch: 'feat', headers: posterHeaders});
  const prIndex = await apiCreatePR(request, poster, repoName, 'feat', 'main', 'review test', {headers: posterHeaders});
  return {repoName, prIndex, pullUrl: `/${poster}/${repoName}/pulls/${prIndex}`};
}

test('pr review comments', async ({page, request}) => {
  const poster = `rv-poster-${randomString(8)}`;
  const reviewer = `rv-reviewer-${randomString(8)}`;
  await Promise.all([apiCreateUser(request, poster), apiCreateUser(request, reviewer)]);
  const {repoName, prIndex, pullUrl} = await createPullRequest(request, poster);

  // reviewer seeds an inline comment via API so the poster's UI reply exercises the reply-to-review path (#35994)
  await Promise.all([
    apiCreateReview(request, poster, repoName, prIndex, {
      comments: [{path: 'added.txt', body: 'inline to reply to', new_position: 1}],
      headers: apiUserHeaders(reviewer),
    }),
    loginUser(page, poster),
  ]);

  await page.goto(`${pullUrl}/files`);

  // diff viewer renders the added file with its header and added-line rows
  const fileBox = page.locator('.diff-file-box[data-new-filename="added.txt"]');
  await expect(fileBox.locator('.diff-file-header .file-link')).toHaveText('added.txt');
  await expect(fileBox.locator('tr.add-code')).toHaveCount(2);

  // commits tab badge reflects the single PR commit, and the diff stats header counts one changed file
  const commitsTab = page.locator('.ui.pull.tabular.menu a.item', {has: page.locator('.octicon-git-commit')});
  await expect(commitsTab.locator('.label')).toHaveText('1');
  await expect(page.locator('.diff-detail-stats')).toContainText(/1 changed file/);

  // poster replies to the reviewer's inline comment
  const conversation = fileBox.locator('.diff-conversation');
  const replyButton = conversation.locator('.diff-viewer-reply');
  await replyButton.click();
  await expect(replyButton).toBeHidden();
  const replyForm = conversation.locator('form');
  await replyForm.getByRole('button', {name: 'Cancel', exact: true}).click();
  await expect(replyForm).toHaveCount(0);
  await replyButton.click();
  await replyForm.locator('textarea[name="content"]').fill('my reply body');
  await replyForm.getByRole('button', {name: 'Reply', exact: true}).click();
  await expect(conversation.locator('.comment-body .markup')).toHaveText(['inline to reply to', 'my reply body']);

  // the conversation tab shows the commented lines as a client-rendered excerpt
  await page.goto(pullUrl);
  await expect(page.locator('[id^="code-preview-"] tr.add-code').first()).toBeVisible();

  // the reviewer, who isn't a collaborator, drafts, cancels, adds and deletes a pending review comment
  await logout(page);
  await loginUser(page, reviewer);
  await page.goto(`${pullUrl}/files`);
  const addLineComment = fileBox.locator('tr.add-code').nth(1).getByRole('button', {name: 'Add line comment', exact: true});
  await addLineComment.click();
  const pendingForm = fileBox.locator('form').filter({has: page.getByRole('button', {name: 'Start review', exact: true})});
  await pendingForm.getByRole('button', {name: 'Cancel', exact: true}).click();
  await expect(pendingForm).toHaveCount(0);
  await addLineComment.click();
  await pendingForm.getByRole('textbox', {name: 'Leave a comment', exact: true}).fill('pending review comment');
  await pendingForm.getByRole('button', {name: 'Start review', exact: true}).click();
  const counter = page.locator('.review-box-button .review-comments-counter');
  await expect(counter).toHaveAttribute('data-pending-comment-number', '1');
  await expect(counter).toHaveText('1');
  const pendingComment = fileBox.locator('.comment').filter({hasText: 'pending review comment'});
  await expect(pendingComment.locator('[data-clipboard-text]')).toHaveAttribute('data-clipboard-text', /\/files#issuecomment-\d+$/);
  await pendingComment.getByRole('menu').filter({has: page.getByRole('menuitem', {name: 'Delete', exact: true, includeHidden: true})}).click();
  await pendingComment.getByRole('menuitem', {name: 'Delete', exact: true}).click();
  await page.getByRole('button', {name: 'Cancel', exact: true}).click();
  await expect(pendingComment).toBeVisible();
  await expect(counter).toHaveAttribute('data-pending-comment-number', '1');
  await pendingComment.getByRole('menu').filter({has: page.getByRole('menuitem', {name: 'Delete', exact: true, includeHidden: true})}).click();
  await pendingComment.getByRole('menuitem', {name: 'Delete', exact: true}).click();
  await page.getByRole('button', {name: 'Confirm', exact: true}).click();
  await expect(pendingComment).toHaveCount(0);
  await expect(counter).toHaveAttribute('data-pending-comment-number', '0');
  await expect(counter).toHaveText('0');
});

test('pr review approvals', async ({page, request}) => {
  const poster = `rv-poster-${randomString(8)}`;
  const reviewer = `rv-reviewer-${randomString(8)}`;
  const officialReviewer = `rv-official-${randomString(8)}`;
  await Promise.all([
    apiCreateUser(request, poster),
    apiCreateUser(request, reviewer),
    apiCreateUser(request, officialReviewer),
  ]);
  const {repoName, prIndex, pullUrl} = await createPullRequest(request, poster);
  await apiAddCollaborator(request, poster, repoName, officialReviewer, {headers: apiUserHeaders(poster)});
  await Promise.all([
    apiCreateReview(request, poster, repoName, prIndex, {event: 'APPROVED', headers: apiUserHeaders(officialReviewer)}),
    loginUser(page, reviewer),
  ]);

  const reRequestAndApprove = async (
    username: string,
    approvalTooltip: string,
    pendingStateClass: string,
    official: boolean,
  ) => {
    await page.goto(pullUrl);
    const reviewerItem = page.locator('.issue-sidebar-combo .ui.relaxed.list > .item', {
      has: page.locator(`a[href="/${username}"]`),
    });
    const reviewState = reviewerItem.locator('span[data-tooltip-content]');
    await expect(reviewState).toHaveAttribute('data-tooltip-content', approvalTooltip);
    await reviewerItem.locator('a[data-tooltip-content="Re-request review"]').click();
    await expect(reviewState).toHaveAttribute('data-tooltip-content', 'Review pending');
    await expect(reviewState.locator(`.octicon-dot-fill.${pendingStateClass}`)).toBeVisible();

    expect(await apiCreateReview(request, poster, repoName, prIndex, {
      event: 'APPROVED',
      headers: apiUserHeaders(username),
    })).toMatchObject({state: 'APPROVED', official});
  };

  await page.goto(`${pullUrl}/files`);
  await page.getByRole('button', {name: 'Review', exact: true}).click();
  const panel = page.locator('.review-box-panel');
  await panel.locator('textarea[name="content"]').fill(`First approval from ${reviewer}`);
  await panel.getByRole('button', {name: 'Approve', exact: true}).click();
  const reviewComment = page.locator('.timeline-item').filter({hasText: `First approval from ${reviewer}`});
  await expect(reviewComment).toBeVisible();
  await expect(reviewComment.locator('[data-clipboard-text]')).toHaveAttribute('data-clipboard-text', /\/pulls\/\d+#issuecomment-\d+$/);

  await reRequestAndApprove(reviewer, 'Uncounted approval', 'tw-text-text-light', false);

  await logout(page);
  await loginUser(page, officialReviewer);
  await reRequestAndApprove(officialReviewer, 'Approved', 'tw-text-yellow', true);
});
