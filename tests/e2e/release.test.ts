import {env} from 'node:process';
import {createHash} from 'node:crypto';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiHeaders, baseUrl, randomString} from './utils.ts';

test('create a release', async ({page, request}) => {
  const repoName = `e2e-release-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([apiCreateRepo(request, {name: repoName}), login(page)]);
  await page.goto(`/${owner}/${repoName}/releases/new`);

  const tag = `v1.0.0-${randomString(8)}`;
  const title = `e2e-release-${randomString(8)}`;
  await page.getByLabel('Tag name').fill(tag);
  await page.getByLabel('Release title').fill(title);
  await page.getByRole('button', {name: 'Publish Release'}).click();

  await page.waitForURL(new RegExp(`/${owner}/${repoName}/releases$`));
  await expect(page.locator('.release-list-title')).toContainText(title);
});

test('show sha256 for release attachments', async ({page, request}) => {
  const repoName = `e2e-release-assets-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const tag = `v1.0.0-${randomString(8)}`;
  const title = `asset-release-${randomString(8)}`;
  const filename = 'checksum.txt';
  const content = Buffer.from(`release attachment ${randomString(16)}`, 'utf8');
  const sha256 = createHash('sha256').update(content).digest('hex');

  await Promise.all([apiCreateRepo(request, {name: repoName}), login(page)]);

  const releaseResponse = await request.post(`${baseUrl()}/api/v1/repos/${owner}/${repoName}/releases`, {
    headers: apiHeaders(),
    data: {tag_name: tag, name: title, body: 'release attachment checksum test'},
  });
  expect(releaseResponse.ok()).toBeTruthy();
  const release = await releaseResponse.json();

  const assetResponse = await request.post(`${baseUrl()}/api/v1/repos/${owner}/${repoName}/releases/${release.id}/assets`, {
    headers: apiHeaders(),
    multipart: {
      attachment: {
        name: filename,
        mimeType: 'text/plain',
        buffer: content,
      },
    },
  });
  expect(assetResponse.ok()).toBeTruthy();

  await page.goto(`/${owner}/${repoName}/releases`);
  const attachmentItem = page.locator('.attachment-list .item').filter({hasText: filename});
  await expect(attachmentItem).toContainText(filename);
  // Button shows the first 12 chars, tooltip holds the full hash
  const checksumBtn = attachmentItem.locator(`button[data-clipboard-text="${sha256}"]`);
  await expect(checksumBtn).toContainText(sha256.slice(0, 12));
});
