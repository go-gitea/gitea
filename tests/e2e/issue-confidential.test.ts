import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateRepo, apiCreateUser, loginUser, login, randomString} from './utils.ts';

test('confidential issue journey: report, hidden from anonymous, make public', async ({page, request, browser}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const reporter = `conf-reporter-${randomString(8)}`;
  const repoName = `e2e-confidential-${randomString(8)}`;
  const title = `secret-${randomString(8)}`;
  await Promise.all([apiCreateUser(request, reporter), apiCreateRepo(request, {name: repoName, autoInit: false})]);

  // 1. the reporter files a confidential issue
  await loginUser(page, reporter);
  await page.goto(`/${owner}/${repoName}/issues/new`);
  await expect(page.getByText('Only you, assignees and people with write access will see this issue.')).toBeVisible();
  await page.getByPlaceholder('Title').fill(title);
  await page.getByPlaceholder('Leave a comment').fill('private details');
  await page.getByLabel('Confidential').check();
  await page.getByRole('button', {name: 'Create Issue'}).click();

  // 2. the issue page carries the marker
  await expect(page).toHaveURL(new RegExp(`/${repoName}/issues/1$`));
  const issueUrl = page.url();
  await expect(page.locator('.issue-state-label').filter({hasText: 'Confidential'})).toBeVisible();

  // 3. an anonymous visitor gets not-found and no list row
  const anonymousContext = await browser.newContext();
  const anonymousPage = await anonymousContext.newPage();
  try {
    const hiddenResponse = await anonymousPage.goto(issueUrl);
    expect(hiddenResponse!.status()).toBe(404);
    await anonymousPage.goto(`/${owner}/${repoName}/issues`);
    await expect(anonymousPage.locator('#issue-list .item').filter({hasText: title})).toHaveCount(0);

    // 4. the writer sees the marked row and makes the issue public
    const writerContext = await browser.newContext();
    const writerPage = await writerContext.newPage();
    try {
      await login(writerPage);
      await writerPage.goto(`/${owner}/${repoName}/issues`);
      const row = writerPage.locator('#issue-list .item').filter({hasText: title});
      await expect(row.getByText('Confidential')).toBeVisible();
      await row.getByRole('link', {name: title}).click();
      await writerPage.getByRole('button', {name: 'Make public'}).click();
      await writerPage.getByRole('dialog').getByRole('button', {name: 'Make public'}).click();

      // 5. the timeline records the change
      await expect(writerPage.getByText('made this issue public')).toBeVisible();
    } finally {
      await writerContext.close();
    }

    // 6. the anonymous visitor can now read the issue
    const publicResponse = await anonymousPage.goto(issueUrl);
    expect(publicResponse!.status()).toBe(200);
    await expect(anonymousPage.getByRole('heading', {name: title})).toBeVisible();
  } finally {
    await anonymousContext.close();
  }
});
