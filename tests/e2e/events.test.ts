import {test, expect} from '@playwright/test';
import {loginUser, apiBaseUrl, apiUserHeaders, apiCreateUser, apiDeleteUser} from './utils.ts';

// These tests rely on EVENT_SOURCE_UPDATE_TIME=1s in the e2e server config.
test.describe('Events', () => {
  test.describe.configure({timeout: 120000});

  test('notification count', async ({page, request}) => {
    const id = `ev-notif-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const owner = `${id}-owner`;
    const commenter = `${id}-commenter`;
    const repoName = id;

    await Promise.all([apiCreateUser(request, owner), apiCreateUser(request, commenter)]);

    // Create repo and issue before login so the notification exists when event stream connects
    await request.post(`${apiBaseUrl()}/api/v1/user/repos`, {
      headers: apiUserHeaders(owner),
      data: {name: repoName, auto_init: true},
    });
    await request.post(`${apiBaseUrl()}/api/v1/repos/${owner}/${repoName}/issues`, {
      headers: apiUserHeaders(commenter),
      data: {title: 'events notification test'},
    });

    // Login as the owner — the first server event poll picks up the notification
    await loginUser(page, owner);

    // Wait for the notification badge to appear via server event
    const badge = page.locator('a.not-mobile .notification_count');
    await expect(badge).toBeVisible({timeout: 60000});

    // Cleanup
    await apiDeleteUser(request, commenter);
    await apiDeleteUser(request, owner);
  });

  test('stopwatch', async ({page, request}) => {
    const name = `ev-sw-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const headers = apiUserHeaders(name);

    await apiCreateUser(request, name);

    // Create repo, issue, and start stopwatch before login
    await request.post(`${apiBaseUrl()}/api/v1/user/repos`, {
      headers,
      data: {name, auto_init: true},
    });
    await request.post(`${apiBaseUrl()}/api/v1/repos/${name}/${name}/issues`, {
      headers,
      data: {title: 'events stopwatch test'},
    });
    await request.post(`${apiBaseUrl()}/api/v1/repos/${name}/${name}/issues/1/stopwatch/start`, {
      headers,
    });

    // Login — page renders with the active stopwatch element
    await loginUser(page, name);

    // Verify stopwatch is visible and links to the correct issue
    const stopwatch = page.locator('.active-stopwatch.not-mobile');
    await expect(stopwatch).toBeVisible();

    // Cleanup
    await apiDeleteUser(request, name);
  });

  test('logout propagation', async ({browser, request}) => {
    const name = `ev-logout-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;

    await apiCreateUser(request, name);

    // Use a single context so both pages share the same session and SharedWorker
    const context = await browser.newContext();
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
    await expect(page2.getByRole('link', {name: 'Sign In'})).toBeVisible({timeout: 60000});

    await context.close();

    // Cleanup
    await apiDeleteUser(request, name);
  });
});
