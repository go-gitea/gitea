import {test, expect} from '@playwright/test';
import {loginUser, apiBaseUrl, apiUserHeaders, apiCreateUser, apiDeleteUser} from './utils.ts';

// These tests rely on EVENT_SOURCE_UPDATE_TIME=2s in the e2e server config.
test.describe('Events', () => {
  test.describe.configure({timeout: 30000});

  test('notification count', async ({page, request}) => {
    const id = `ev-notif-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const owner = `${id}-owner`;
    const commenter = `${id}-commenter`;
    const repoName = id;

    await Promise.all([apiCreateUser(request, owner), apiCreateUser(request, commenter)]);

    // Create a repo owned by the dedicated user
    await request.post(`${apiBaseUrl()}/api/v1/user/repos`, {
      headers: apiUserHeaders(owner),
      data: {name: repoName, auto_init: true},
    });

    // Login as the owner so the event connection starts
    await loginUser(page, owner);

    // Verify notification badge is initially hidden (use desktop variant)
    const badge = page.locator('a.not-mobile .notification_count');
    await expect(badge).toBeHidden();

    // Create an issue as the commenter to generate a notification for the owner
    await request.post(`${apiBaseUrl()}/api/v1/repos/${owner}/${repoName}/issues`, {
      headers: apiUserHeaders(commenter),
      data: {title: 'events notification test'},
    });

    // Wait for the notification badge to appear via server event
    await expect(badge).toBeVisible({timeout: 20000});

    // Cleanup
    await apiDeleteUser(request, commenter);
    await apiDeleteUser(request, owner);
  });

  test('stopwatch', async ({page, request}) => {
    const name = `ev-sw-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const headers = apiUserHeaders(name);

    await apiCreateUser(request, name);

    // Create a repo and issue owned by the dedicated user
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

    // Login as the dedicated user
    await loginUser(page, name);

    // Listen for a stopwatch event via a direct EventSource connection
    const stopwatchData = await page.evaluate(() => {
      return new Promise<any[]>((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('stopwatch event timeout')), 15000);
        const es = new EventSource('/user/events');
        es.addEventListener('stopwatches', (e: MessageEvent) => {
          clearTimeout(timeout);
          es.close();
          resolve(JSON.parse(e.data));
        });
        es.addEventListener('error', () => {
          clearTimeout(timeout);
          es.close();
          reject(new Error('EventSource connection error'));
        });
      });
    });

    expect(stopwatchData).toHaveLength(1);
    expect(stopwatchData[0].repo_owner_name).toBe(name);
    expect(stopwatchData[0].repo_name).toBe(name);
    expect(stopwatchData[0].issue_index).toBe(1);

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

    // Navigate page2 so the SharedWorker connects on both pages
    await page2.goto('/');
    await page2.waitForTimeout(1000);

    // Verify page2 is logged in
    await expect(page2.getByRole('link', {name: 'Sign In'})).toBeHidden();

    // Logout from page1 — this sends a logout event
    await page1.goto('/user/logout');

    // page2 should be redirected via logout event
    // (logoutFromWorker waits 5s before redirecting)
    await expect(page2.getByRole('link', {name: 'Sign In'})).toBeVisible({timeout: 15000});

    await context.close();

    // Cleanup
    await apiDeleteUser(request, name);
  });
});
