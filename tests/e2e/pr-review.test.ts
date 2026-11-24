import {createHash} from 'node:crypto';
import {test, expect} from '@playwright/test';
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

test('pr review code suggestion', async ({page, request}) => {
  const poster = `rv-sugg-poster-${randomString(8)}`;
  const reviewer = `rv-sugg-reviewer-${randomString(8)}`;
  await Promise.all([apiCreateUser(request, poster), apiCreateUser(request, reviewer)]);
  const posterHeaders = apiUserHeaders(poster);
  const repoName = `e2e-prsugg-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName, headers: posterHeaders});
  await apiCreateFiles(request, poster, repoName, [{path: 'added.txt', content: 'first\nsecond\nthird\n'}], {branch: 'main', newBranch: 'feat', headers: posterHeaders});
  const prIndex = await apiCreatePR(request, poster, repoName, 'feat', 'main', 'suggestion test', {headers: posterHeaders});
  await Promise.all([
    apiCreateReview(request, poster, repoName, prIndex, {
      comments: [{path: 'added.txt', body: '```suggestion\nsecond line\n```', new_position: 2}],
      headers: apiUserHeaders(reviewer),
    }),
    loginUser(page, poster),
  ]);

  await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);
  const fileBox = page.locator('.diff-file-box[data-new-filename="added.txt"]');
  const addedRows = fileBox.locator('table[data-new-comment-url] > tbody > tr[data-line-type="add"]');
  const suggestion = fileBox.locator('.code-suggestion');
  await expect(suggestion.locator('tr.del-code')).toHaveText(['2-second']);
  await expect(suggestion.locator('tr.add-code')).toHaveText(['2+second line']);

  await addedRows.first().getByRole('button', {name: 'Add line comment', exact: true}).click();
  const commentForm = fileBox.locator('form').filter({has: page.getByRole('button', {name: 'Start review', exact: true})});
  await commentForm.getByRole('button', {name: 'Add a suggestion', exact: true}).click();
  const editor = commentForm.getByRole('textbox', {name: 'Leave a comment', exact: true});
  await expect(editor).toHaveValue('```suggestion\nfirst\n```');
  await editor.fill('```suggestion\nfirst line\n```');
  await commentForm.getByText('Preview', {exact: true}).click();
  const preview = commentForm.locator('.code-suggestion');
  await expect(preview).toBeVisible();
  await expect(preview.locator('tr.del-code')).toHaveText(['1-first']);
  await expect(preview.locator('tr.add-code')).toHaveText(['1+first line']);
  await commentForm.getByRole('button', {name: 'Cancel', exact: true}).click();

  await fileBox.getByRole('button', {name: 'Apply suggestion', exact: true}).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('textbox', {name: 'Commit summary', exact: true})).toHaveValue(`Apply suggestion from @${reviewer}`);
  await dialog.getByRole('button', {name: 'Commit suggestion', exact: true}).click();
  await expect(addedRows.locator('> .lines-code .code-inner')).toHaveText(['first', 'second line', 'third']);
});

test('pr diff line selection and links', async ({page, request}) => {
  const poster = `rv-lines-poster-${randomString(8)}`;
  const reviewer = `rv-lines-reviewer-${randomString(8)}`;
  await Promise.all([apiCreateUser(request, poster), apiCreateUser(request, reviewer)]);
  const headers = apiUserHeaders(poster);
  const repoName = `e2e-prlines-${randomString(8)}`;
  const filePath = 'added.txt';
  const [prIndex] = await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, headers});
      await apiCreateFiles(request, poster, repoName, [{path: filePath, content: 'first\nsecond\nthird\nfourth\nfifth\n'}], {branch: 'main', newBranch: 'feat', headers});
      return apiCreatePR(request, poster, repoName, 'feat', 'main', 'line selection test', {headers});
    })(),
    loginUser(page, reviewer),
  ]);

  await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);
  const fileBox = page.locator(`.diff-file-box[data-new-filename="${filePath}"]`);
  const firstLine = page.locator('td.lines-num-new').getByRole('link', {name: '2', exact: true});
  const lastLine = page.locator('td.lines-num-new').getByRole('link', {name: '4', exact: true});
  const fileHash = createHash('sha1').update(filePath).digest('hex');
  const highlightedRows = fileBox.locator('tr.diff-range-highlight');

  await fileBox.locator(firstLine).click();
  await expect(page).toHaveURL((url) => url.hash === `#diff-${fileHash}R2`);
  await expect(highlightedRows).toHaveText([/second/]);
  await fileBox.locator(lastLine).click({modifiers: ['Shift']});
  await expect(page).toHaveURL((url) => url.hash === `#diff-${fileHash}R2-R4`);
  await expect(highlightedRows).toHaveText([/second/, /third/, /fourth/]);

  await page.reload();
  await expect(highlightedRows).toHaveText([/second/, /third/, /fourth/]);

  await fileBox.getByRole('row').filter({has: lastLine}).getByRole('button', {name: 'Add line comment', exact: true}).click();
  const draft = fileBox.locator('.diff-comment-draft');
  const rangeLabel = 'Lines 2 to 4';
  await expect(draft.getByText(rangeLabel, {exact: true})).toBeVisible();
  const comment = 'Reviewing these lines together.';
  await draft.getByRole('textbox', {name: 'Leave a comment', exact: true}).fill(comment);
  await draft.getByRole('button', {name: 'Add single comment', exact: true}).click();
  const thread = fileBox.locator('.comment-code-cloud');
  await expect(thread.locator('.comment-body')).toContainText(comment);
  await expect(thread.getByText(rangeLabel, {exact: true})).toBeVisible();
});
