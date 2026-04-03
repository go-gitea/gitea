import {test, expect} from '@playwright/test';
import {loginUser, baseUrl, apiUserHeaders, apiCreateUser, apiDeleteUser, apiCreateRepo, apiCreateIssue, apiStartStopwatch, timeoutFactor, randomString} from './utils.ts';

// These tests rely on a short EVENT_SOURCE_UPDATE_TIME in the e2e server config.
test.describe('events', () => {
  test('notification count', async ({page, request}) => {
    const owner = `ev-notif-owner-${randomString(8)}`;
    const commenter = `ev-notif-commenter-${randomString(8)}`;
    const repoName = `ev-notif-${randomString(8)}`;

    await Promise.all([apiCreateUser(request, owner), apiCreateUser(request, commenter)]);

    // Create repo and login in parallel — repo is needed for the issue, login for the event stream
    await Promise.all([
      apiCreateRepo(request, {name: repoName, headers: apiUserHeaders(owner)}),
      loginUser(page, owner),
    ]);
    const badge = page.locator('a.not-mobile .notification_count');
    await expect(badge).toBeHidden();

    // Create issue as another user — this generates a notification delivered via server push
    await apiCreateIssue(request, owner, repoName, {title: 'events notification test', headers: apiUserHeaders(commenter)});

    // Wait for the notification badge to appear via server event
    await expect(badge).toBeVisible({timeout: 15000 * timeoutFactor});

    // Cleanup
    await Promise.all([apiDeleteUser(request, commenter), apiDeleteUser(request, owner)]);
  });

  test('stopwatch', async ({page, request}) => {
    const name = `ev-sw-${randomString(8)}`;
    const headers = apiUserHeaders(name);

    await apiCreateUser(request, name);

    // Create repo, issue, and start stopwatch before login
    await apiCreateRepo(request, {name, headers});
    await apiCreateIssue(request, name, name, {title: 'events stopwatch test', headers});
    await apiStartStopwatch(request, name, name, 1, {headers});

    // Login — page renders with the active stopwatch element
    await loginUser(page, name);

    // Verify stopwatch is visible and links to the correct issue
    const stopwatch = page.locator('.active-stopwatch.not-mobile');
    await expect(stopwatch).toBeVisible();

    // Cleanup
    await apiDeleteUser(request, name);
  });

  test('logout propagation', async ({browser, request}) => {
    const name = `ev-logout-${randomString(8)}`;

    await apiCreateUser(request, name);

    // Use a single context so both pages share the same session and SharedWorker
    const context = await browser.newContext({baseURL: baseUrl()});
    const page1 = await context.newPage();
    const page2 = await context.newPage();

    await loginUser(page1, name);

    // Navigate page2 so it connects to the shared event stream
    await page2.goto('/');

    // Verify page2 is logged in
    await expect(page2.getByRole('link', {name: 'Sign In'})).toBeHidden();

    // Logout from page1 — this sends a logout event to all tabs
    await page1.goto('/user/logout');

    // page2 should be redirected via the logout event
    await expect(page2.getByRole('link', {name: 'Sign In'})).toBeVisible();

    await context.close();

    // Cleanup
    await apiDeleteUser(request, name);
  });
});
