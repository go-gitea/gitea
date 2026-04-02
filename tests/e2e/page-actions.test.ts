import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo, apiCreateUser, apiDeleteUser, apiCreateIssue, apiUserHeaders, assertNoJsError, timeoutFactor} from './utils.ts';

test('star and unstar repo', async ({page, request}) => {
  const repoName = `e2e-star-${Date.now()}`;
  await apiCreateRepo(request, {name: repoName});
  try {
    await login(page);
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}`);

    const starForm = () => page.locator('form[action$="/action/star"], form[action$="/action/unstar"]');

    await expect(starForm().locator('button[aria-label="Star"]')).toBeVisible();
    await expect(starForm().locator(`a[href$="/stars"]`)).toHaveText('0');

    await starForm().locator('button').click();
    await expect(starForm().locator('button[aria-label="Unstar"]')).toBeVisible();
    await expect(starForm().locator(`a[href$="/stars"]`)).toHaveText('1');

    await starForm().locator('button').click();
    await expect(starForm().locator('button[aria-label="Star"]')).toBeVisible();
    await expect(starForm().locator(`a[href$="/stars"]`)).toHaveText('0');
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('watch and unwatch repo', async ({page, request}) => {
  const repoName = `e2e-watch-${Date.now()}`;
  await apiCreateRepo(request, {name: repoName});
  try {
    await login(page);
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}`);

    const watchForm = () => page.locator('form[action$="/action/watch"], form[action$="/action/unwatch"]');

    // Repo owner auto-watches, so initial state is "Unwatch"
    await expect(watchForm().locator('button[aria-label="Unwatch"]')).toBeVisible();

    await watchForm().locator('button').click();
    await expect(watchForm().locator('button[aria-label="Watch"]')).toBeVisible();

    await watchForm().locator('button').click();
    await expect(watchForm().locator('button[aria-label="Unwatch"]')).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('issue subscribe toggle', async ({page, request}) => {
  const repoName = `e2e-issuewatch-${Date.now()}`;
  await apiCreateRepo(request, {name: repoName});
  try {
    await login(page);
    await apiCreateIssue(request, env.GITEA_TEST_E2E_USER, repoName, {title: 'test issue'});
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/1`);

    await expect(page.locator('.issue-content-right form button:has-text("Unsubscribe")')).toBeVisible();

    await page.locator('.issue-content-right form button:has-text("Unsubscribe")').click();
    await expect(page.locator('.issue-content-right form button:has-text("Subscribe")')).toBeVisible();

    await page.locator('.issue-content-right form button:has-text("Subscribe")').click();
    await expect(page.locator('.issue-content-right form button:has-text("Unsubscribe")')).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('follow and unfollow user', async ({page, request}) => {
  const targetUser = `e2e-follow-${Date.now()}`;
  await apiCreateUser(request, targetUser);
  try {
    await login(page);
    await page.goto(`/${targetUser}`);

    await expect(page.locator('#profile-avatar-card button:has-text("Follow")')).toBeVisible();

    await page.locator('#profile-avatar-card button:has-text("Follow")').click();
    await expect(page.locator('#profile-avatar-card button:has-text("Unfollow")')).toBeVisible();

    await page.locator('#profile-avatar-card button:has-text("Unfollow")').click();
    await expect(page.locator('#profile-avatar-card button:has-text("Follow")')).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteUser(request, targetUser);
  }
});

test('notification pin', async ({page, request}) => {
  const commenter = `e2e-notif-${Date.now()}`;
  const repoName = `e2e-notif-${Date.now()}`;
  await apiCreateUser(request, commenter);
  await apiCreateRepo(request, {name: repoName});
  try {
    await apiCreateIssue(request, env.GITEA_TEST_E2E_USER, repoName, {
      title: 'notification test issue',
      headers: apiUserHeaders(commenter),
    });

    await login(page);
    await page.goto('/notifications');

    const notificationItem = page.locator('.notifications-item').first();
    await expect(notificationItem).toBeVisible();

    await notificationItem.hover();
    await notificationItem.locator('button[name="notification_action"][value="pin"]').click();
    await expect(page.locator('#notification_div')).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteUser(request, commenter);
    await apiDeleteRepo(request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('admin system status polling', async ({page}) => {
  await login(page);
  await page.goto('/-/admin');

  const goroutinesLabel = page.locator('dt:has-text("Current Goroutines")');
  await expect(goroutinesLabel).toBeVisible();
  await expect(goroutinesLabel.locator('+ dd')).toHaveText(/\d+/);

  await page.waitForResponse((resp) => resp.url().includes('/system_status') && resp.ok(), {timeout: 10000 * timeoutFactor});
  await expect(goroutinesLabel).toBeVisible();
  await expect(goroutinesLabel.locator('+ dd')).toHaveText(/\d+/);
  await assertNoJsError(page);
});

test('repo file list commit messages', async ({page, request}) => {
  const repoName = `e2e-commits-${Date.now()}`;
  await apiCreateRepo(request, {name: repoName});
  try {
    await login(page);
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}`);

    const fileTable = page.locator('#repo-files-table');
    await expect(fileTable).toBeVisible();
    await expect(fileTable.locator('.repo-file-cell.message a').first()).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('stargazers page refreshes on star', async ({page, request}) => {
  const repoName = `e2e-cards-${Date.now()}`;
  await apiCreateRepo(request, {name: repoName});
  try {
    await login(page);
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/stars`);

    const userCards = page.locator('.user-cards');
    await expect(userCards).toBeVisible();

    await page.locator('form[action$="/action/star"], form[action$="/action/unstar"]').locator('button').click();
    await expect(userCards.locator('.item')).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('editor diff preview', async ({page, request}) => {
  const repoName = `e2e-editor-${Date.now()}`;
  await apiCreateRepo(request, {name: repoName});
  try {
    await login(page);
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/_edit/main/README.md`);

    await expect(page.locator('.editor-loading')).toBeHidden();
    const editor = page.locator('.cm-content[role="textbox"]');
    await expect(editor).toBeVisible();
    await editor.click();
    await page.keyboard.type('\nNew line added for diff test');

    const diffTab = page.locator('a[data-tab="diff"]');
    await expect(diffTab).toBeVisible();
    await diffTab.click();

    await expect(page.locator('.tab[data-tab="diff"]').locator('.diff-file-box, .diff-file-header, .markup')).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, env.GITEA_TEST_E2E_USER, repoName);
  }
});
