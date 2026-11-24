import {createHash} from 'node:crypto';
import {env} from 'node:process';
import {test, expect, type APIRequestContext} from '@playwright/test';
import {
  apiAddCollaborator,
  apiCreateFiles,
  apiCreatePR,
  apiCreateRepo,
  apiCreateReview,
  apiCreateUser,
  apiUserHeaders,
  login,
  loginUser,
  logout,
  randomString,
} from './utils.ts';

test('pr review flow', async ({page, request}) => {
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
      [{path: 'added.txt', content: 'new content\nsecond line\n'}],
      {branch: 'main', newBranch: 'feat', headers: posterHeaders},
    ),
  ]);
  const prIndex = await apiCreatePR(request, poster, repoName, 'feat', 'main', 'review test', {headers: posterHeaders});
  const pullUrl = `/${poster}/${repoName}/pulls/${prIndex}`;

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

  // diff viewer renders the added file with its header and added-line rows
  const fileBox = page.locator('.diff-file-box[data-new-filename="added.txt"]');
  await expect(fileBox.locator('.diff-file-header .file-link')).toHaveText('added.txt');
  await expect(fileBox.locator('tr.add-code')).toHaveCount(2);

  // commits tab badge reflects the single PR commit, and the diff stats header counts one changed file
  const commitsTab = page.locator('.ui.pull.tabular.menu a.item', {has: page.locator('.octicon-git-commit')});
  await expect(commitsTab.locator('.label')).toHaveText('1');
  await expect(page.locator('.diff-detail-stats')).toContainText(/1 changed file/);

  // poster replies to the reviewer's inline comment
  const conversation = fileBox.locator('.conversation-holder');
  await conversation.locator('.comment-form-reply').click();
  const replyForm = conversation.locator('form');
  await replyForm.getByRole('button', {name: 'Cancel', exact: true}).click();
  await expect(replyForm).toBeHidden();
  await conversation.locator('.comment-form-reply').click();
  await replyForm.locator('textarea[name="content"]').fill('my reply body');
  await replyForm.getByRole('button', {name: 'Reply', exact: true}).click();
  await expect(conversation.locator('.comment-body')).toContainText(['inline to reply to', 'my reply body']);

  await logout(page);
  await loginUser(page, reviewer);
  await page.goto(`${pullUrl}/files`);
  await fileBox.locator('tr.add-code').nth(1).getByRole('button', {name: 'Add line comment', exact: true}).click();
  const pendingForm = fileBox.locator('form').filter({has: page.getByRole('button', {name: 'Start review', exact: true})});
  await pendingForm.getByRole('button', {name: 'Cancel', exact: true}).click();
  await expect(pendingForm).toHaveCount(0);
  await fileBox.locator('tr.add-code').nth(1).getByRole('button', {name: 'Add line comment', exact: true}).click();
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

const reviewFile = 'added.txt';

async function createReviewDiff(request: APIRequestContext) {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `e2e-review-diff-${randomString(8)}`;
  await apiCreateRepo(request, {name: repo});
  await apiCreateFiles(request, owner, repo, [{path: reviewFile, content: 'first\nsecond\nthird\nfourth\n'}], {branch: 'main', newBranch: 'feat'});
  const index = await apiCreatePR(request, owner, repo, 'feat', 'main', 'review diff');
  return {owner, repo, index, url: `/${owner}/${repo}/pulls/${index}/files`};
}

test('previews and applies a code suggestion from the PR diff', async ({page, request}) => {
  const reviewer = `rv-sugg-reviewer-${randomString(8)}`;
  const [{owner, repo, index, url}] = await Promise.all([createReviewDiff(request), apiCreateUser(request, reviewer), login(page)]);
  await apiCreateReview(request, owner, repo, index, {
    comments: [{path: reviewFile, body: '```suggestion\nsecond revised\n```', new_position: 2}],
    headers: apiUserHeaders(reviewer),
  });

  await page.goto(url);
  const file = page.locator(`.diff-file-box[data-new-filename="${reviewFile}"]`);
  await expect(file.locator('.code-suggestion .added-code')).toHaveText('revised');
  await file.getByRole('button', {name: 'Add line comment', exact: true}).first().click();
  const draft = file.locator('.diff-comment-draft');
  const addSuggestion = draft.getByRole('button', {name: 'Add a suggestion', exact: true});
  const editor = draft.getByRole('textbox', {name: 'Leave a comment', exact: true});
  await addSuggestion.click();
  await expect(editor).toHaveValue('```suggestion\nfirst\n```');
  await editor.fill('```suggestion\nfirst revised\n```');
  await draft.getByText('Preview', {exact: true}).click();
  await expect(draft.locator('.code-suggestion .added-code')).toHaveText('revised');
  await draft.getByText('Write', {exact: true}).click();
  await editor.clear();
  await draft.locator('[data-tooltip-content="Use the legacy editor instead"]').click();
  await draft.locator('.editor-toolbar').getByRole('button', {name: 'Add a suggestion', exact: true}).click();
  await expect(draft.locator('textarea[name="content"]')).toHaveValue('```suggestion\nfirst\n```');
  await draft.getByRole('button', {name: 'Cancel', exact: true}).click();

  await file.getByRole('button', {name: 'Apply suggestion', exact: true}).click();
  await page.getByRole('dialog').getByRole('button', {name: 'Commit suggestion', exact: true}).click();
  await expect(file.locator('table[data-new-comment-url] > tbody > tr.add-code > .lines-code .code-inner')).toHaveText([
    'first', 'second revised', 'third', 'fourth',
  ]);
});

test('pr diff line selection and links', async ({page, request}) => {
  const [{url}] = await Promise.all([createReviewDiff(request), login(page)]);
  await page.goto(url);
  const file = page.locator(`.diff-file-box[data-new-filename="${reviewFile}"]`);
  const proposed = file.locator('td.lines-num-new');
  const lastLine = page.getByRole('link', {name: '4', exact: true});
  const fileHash = createHash('sha1').update(reviewFile).digest('hex');

  await proposed.getByRole('link', {name: '2', exact: true}).click();
  await proposed.locator(lastLine).click({modifiers: ['Shift']});
  await expect(page).toHaveURL((url) => url.hash === `#diff-${fileHash}R2-R4`);
  await page.reload();
  await expect(file.locator('tr.diff-range-highlight')).toHaveText([/second/, /third/, /fourth/]);

  await file.getByRole('row').filter({has: lastLine})
    .getByRole('button', {name: 'Add line comment', exact: true}).click();
  const draft = file.locator('.diff-comment-draft');
  await expect(draft.getByText('Lines 2 to 4', {exact: true})).toBeVisible();
  await draft.getByRole('textbox', {name: 'Leave a comment', exact: true}).fill('Reviewing these lines together.');
  await draft.getByRole('button', {name: 'Add single comment', exact: true}).click();
  const thread = file.locator('.comment-code-cloud').filter({hasText: 'Reviewing these lines together.'});
  await expect(thread.getByText('Lines 2 to 4', {exact: true})).toBeVisible();
});
