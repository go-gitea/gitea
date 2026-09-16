import {test, expect, type Page} from '@playwright/test';
import {
  apiAddCollaborator,
  apiCreateFiles,
  apiCreatePR,
  apiCreateRepo,
  apiCreateReview,
  apiCreateUser,
  apiUserHeaders,
  loginUser,
  randomString,
} from './utils.ts';

test('pr review flow', async ({browser, page, request}) => {
  const poster = `rv-poster-${randomString(8)}`;
  const reviewer = `rv-reviewer-${randomString(8)}`;
  const officialReviewer = `rv-official-${randomString(8)}`;
  await Promise.all([
    apiCreateUser(request, poster),
    apiCreateUser(request, reviewer),
    apiCreateUser(request, officialReviewer),
  ]);
  const posterHeaders = apiUserHeaders(poster);
  const repoName = `e2e-prreview-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName, headers: posterHeaders});
  await Promise.all([
    apiAddCollaborator(
      request,
      poster,
      repoName,
      officialReviewer,
      {headers: posterHeaders},
    ),
    apiCreateFiles(
      request,
      poster,
      repoName,
      [{path: 'added.txt', content: 'new content\n'}],
      {branch: 'main', newBranch: 'feat', headers: posterHeaders},
    ),
  ]);
  const prIndex = await apiCreatePR(request, poster, repoName, 'feat', 'main', 'review test', {headers: posterHeaders});
  const pullUrl = `/${poster}/${repoName}/pulls/${prIndex}`;

  const loginInNewContext = async (username: string) => {
    const userPage = await (await browser.newContext()).newPage();
    await loginUser(userPage, username);
    return userPage;
  };

  const reRequestAndApprove = async (
    userPage: Page,
    username: string,
    approvalTooltip: string,
    pendingStateClass: string,
    official: boolean,
  ) => {
    await userPage.goto(pullUrl);
    const reviewerItem = userPage.locator('.issue-sidebar-combo .ui.relaxed.list > .item', {
      has: userPage.locator(`a[href="/${username}"]`),
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

  // reviewer seeds an inline comment via API so the poster's UI reply exercises the reply-to-review path (#35994)
  await Promise.all([
    apiCreateReview(request, poster, repoName, prIndex, {
      comments: [{path: 'added.txt', body: 'inline to reply to', new_position: 1}],
      headers: apiUserHeaders(reviewer),
    }),
    apiCreateReview(request, poster, repoName, prIndex, {
      event: 'APPROVED',
      headers: apiUserHeaders(officialReviewer),
    }),
    loginUser(page, poster),
  ]);

  await page.goto(`${pullUrl}/files`);

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

  const reviewerPage = await loginInNewContext(reviewer);
  await reviewerPage.goto(`${pullUrl}/files`);
  await reviewerPage.locator('#review-box .js-btn-review').click();
  const panel = reviewerPage.locator('.review-box-panel');
  await panel.locator('textarea[name="content"]').fill(`First approval from ${reviewer}`);
  await panel.getByRole('button', {name: 'Approve', exact: true}).click();
  await expect(reviewerPage.locator('.timeline-item').filter({hasText: `First approval from ${reviewer}`})).toBeVisible();

  await reRequestAndApprove(reviewerPage, reviewer, 'Uncounted approval', 'tw-text-text-light', false);

  const officialPage = await loginInNewContext(officialReviewer);
  await reRequestAndApprove(officialPage, officialReviewer, 'Approved', 'tw-text-yellow', true);
  await Promise.all([reviewerPage.context().close(), officialPage.context().close()]);
});
