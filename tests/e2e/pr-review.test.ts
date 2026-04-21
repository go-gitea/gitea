import {test, expect} from '@playwright/test';
import type {APIRequestContext} from '@playwright/test';
import {apiCreateFile, apiCreatePR, apiCreateRepo, apiCreateReview, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

async function createReviewUsers(request: APIRequestContext) {
  const poster = `rv-poster-${randomString(8)}`;
  const reviewer = `rv-reviewer-${randomString(8)}`;
  await Promise.all([apiCreateUser(request, poster), apiCreateUser(request, reviewer)]);
  return {poster, reviewer};
}

/** Build a PR owned by `poster` — reviewer exists as a separate user to avoid self-review restrictions. */
async function createReviewablePR(request: APIRequestContext, poster: string) {
  const repoName = `e2e-prreview-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName, headers: apiUserHeaders(poster)});
  await apiCreateFile(request, poster, repoName, 'added.txt', 'new content\n', {branch: 'main', newBranch: 'feat'});
  const prIndex = await apiCreatePR(request, poster, repoName, 'feat', 'main', 'review test');
  return {repoName, prIndex};
}

test.describe('pr review', () => {
  test('top-level comment', async ({page, request}) => {
    const {poster, reviewer} = await createReviewUsers(request);
    const [, {repoName, prIndex}] = await Promise.all([loginUser(page, reviewer), createReviewablePR(request, poster)]);
    await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);

    await page.locator('#review-box .js-btn-review').click();
    const panel = page.locator('.review-box-panel');
    await panel.locator('textarea[name="content"]').fill('looks fine');
    await panel.getByRole('button', {name: 'Comment', exact: true}).click();

    await expect(page.locator('.timeline-item.comment').filter({hasText: 'looks fine'})).toBeVisible();
  });

  test('reply to inline comment', async ({page, request}) => {
    // regression for #35994 "Replying a code review results in 500"
    const {poster, reviewer} = await createReviewUsers(request);
    const [, {repoName, prIndex}] = await Promise.all([loginUser(page, poster), createReviewablePR(request, poster)]);
    await apiCreateReview(request, poster, repoName, prIndex, {
      comments: [{path: 'added.txt', body: 'inline to reply to', new_position: 1}],
      headers: apiUserHeaders(reviewer),
    });
    await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);

    const conversation = page.locator('.diff-file-box[data-new-filename="added.txt"] .conversation-holder');
    await conversation.locator('.comment-form-reply').click();
    const replyForm = conversation.locator('form');
    await replyForm.locator('textarea[name="content"]').fill('my reply body');
    await replyForm.getByRole('button', {name: 'Reply', exact: true}).click();

    await expect(conversation.locator('.comment-body')).toContainText(['inline to reply to', 'my reply body']);
  });

  test('approve review', async ({page, request}) => {
    const {poster, reviewer} = await createReviewUsers(request);
    const [, {repoName, prIndex}] = await Promise.all([loginUser(page, reviewer), createReviewablePR(request, poster)]);
    await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);

    await page.locator('#review-box .js-btn-review').click();
    const panel = page.locator('.review-box-panel');
    await panel.locator('textarea[name="content"]').fill('LGTM');
    await panel.getByRole('button', {name: 'Approve', exact: true}).click();

    await expect(page.locator('.timeline-item .octicon-check').first()).toBeVisible();
    await expect(page.locator('.timeline-item').filter({hasText: 'LGTM'})).toBeVisible();
  });

  test('self-review disabled', async ({page, request}) => {
    const poster = `rv-self-${randomString(8)}`;
    await apiCreateUser(request, poster);
    const posterHeaders = apiUserHeaders(poster);
    const repoName = `e2e-prreview-self-${randomString(8)}`;
    // login can run in parallel with repo/PR setup once the poster user exists
    const [, prIndex] = await Promise.all([
      loginUser(page, poster),
      (async () => {
        await apiCreateRepo(request, {name: repoName, headers: posterHeaders});
        await apiCreateFile(request, poster, repoName, 'added.txt', 'new\n', {branch: 'main', newBranch: 'feat'});
        // poster must be the PR author for self-review to trigger
        return apiCreatePR(request, poster, repoName, 'feat', 'main', 'self-review', {headers: posterHeaders});
      })(),
    ]);

    await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);
    await page.locator('#review-box .js-btn-review').click();

    await expect(page.locator('.review-box-panel button[name="type"][value="approve"]')).toBeDisabled();
  });

  test('request changes review', async ({page, request}) => {
    const {poster, reviewer} = await createReviewUsers(request);
    const [, {repoName, prIndex}] = await Promise.all([loginUser(page, reviewer), createReviewablePR(request, poster)]);
    await page.goto(`/${poster}/${repoName}/pulls/${prIndex}/files`);

    await page.locator('#review-box .js-btn-review').click();
    const panel = page.locator('.review-box-panel');
    await panel.locator('textarea[name="content"]').fill('needs changes');
    await panel.getByRole('button', {name: 'Request changes', exact: true}).click();

    await expect(page.locator('.timeline-item').filter({hasText: 'needs changes'})).toBeVisible();
    // ReviewTypeReject renders as octicon-diff on a red badge (see ReviewType.Icon())
    await expect(page.locator('.timeline-item-group .badge.tw-bg-red .octicon-diff')).toBeVisible();
  });
});
