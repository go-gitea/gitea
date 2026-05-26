import {test, expect} from '@playwright/test';
import {loginUser, baseUrl, apiUserHeaders, apiCreateUser, apiCreateRepo, apiCreateIssue, apiStartStopwatch, apiCancelStopwatch, timeoutFactor, randomString} from './utils.ts';

// The /-/ws WebSocket pipeline is push-only: every event is fired by the server
// immediately on the DB write. These tests exercise that each event type
// (notification-count, stopwatches, logout) reaches a connected tab.
test.describe('events', () => {
  test('notification count increases on new notification', async ({page, request}) => {
    const owner = `ev-notif-owner-${randomString(8)}`;
    const commenter = `ev-notif-commenter-${randomString(8)}`;
    const repoName = `ev-notif-${randomString(8)}`;

    await Promise.all([apiCreateUser(request, owner), apiCreateUser(request, commenter)]);

    await Promise.all([
      apiCreateRepo(request, {name: repoName, autoInit: false, headers: apiUserHeaders(owner)}),
      loginUser(page, owner),
    ]);
    await page.goto('/');
    const badge = page.locator('a.not-mobile .notification_count');
    await expect(badge).toBeHidden();

    // Pushes fired before the SharedWorker subscribes are dropped, so retry
    // the trigger until one lands. Extra notifications are harmless here.
    const commenterHeaders = apiUserHeaders(commenter);
    await expect.poll(async () => {
      await apiCreateIssue(request, {owner, repo: repoName, title: `events-notif-${Date.now()}`, headers: commenterHeaders});
      await page.waitForTimeout(300 * timeoutFactor); // eslint-disable-line playwright/no-wait-for-timeout
      return await badge.isVisible();
    }, {timeout: 10000 * timeoutFactor, intervals: [0]}).toBe(true);
  });

  test('stopwatch appears on active-at-page-load', async ({page, request}) => {
    const name = `ev-sw-${randomString(8)}`;
    const headers = apiUserHeaders(name);

    await apiCreateUser(request, name);
    await Promise.all([
      loginUser(page, name),
      (async () => {
        await apiCreateRepo(request, {name, autoInit: false, headers});
        await apiCreateIssue(request, {owner: name, repo: name, title: 'events stopwatch test', headers});
        await apiStartStopwatch(request, name, name, 1, {headers});
      })(),
    ]);
    await page.goto('/');

    const stopwatch = page.locator('.active-stopwatch.not-mobile');
    await expect(stopwatch).toBeVisible();
  });

  test('stopwatch appears via real-time push', async ({page, request}) => {
    const name = `ev-sw-push-${randomString(8)}`;
    const headers = apiUserHeaders(name);

    await apiCreateUser(request, name);
    await Promise.all([
      loginUser(page, name),
      (async () => {
        await apiCreateRepo(request, {name, headers});
        await apiCreateIssue(request, {owner: name, repo: name, title: 'events stopwatch push test', headers});
      })(),
    ]);
    // Page loads before the stopwatch starts — the icon is hidden in the rendered HTML
    await page.goto('/');
    const stopwatch = page.locator('.active-stopwatch.not-mobile');
    // Element must exist in the DOM (just hidden); otherwise the push has nothing to reveal.
    await expect(stopwatch).toHaveCount(1);
    await expect(stopwatch).toBeHidden();

    // Wait for the SharedWorker WS to subscribe; pushes before that are dropped.
    await page.waitForTimeout(1000 * timeoutFactor); // eslint-disable-line playwright/no-wait-for-timeout

    // Start the stopwatch from outside this tab; the push should reveal the icon
    await apiStartStopwatch(request, name, name, 1, {headers});
    await expect(stopwatch).toBeVisible({timeout: 5000 * timeoutFactor});
  });

  test('stopwatch hides via real-time push on cancel', async ({page, request}) => {
    const name = `ev-sw-stop-${randomString(8)}`;
    const headers = apiUserHeaders(name);

    await apiCreateUser(request, name);
    await Promise.all([
      loginUser(page, name),
      (async () => {
        await apiCreateRepo(request, {name, headers});
        await apiCreateIssue(request, {owner: name, repo: name, title: 'events stopwatch stop test', headers});
        await apiStartStopwatch(request, name, name, 1, {headers});
      })(),
    ]);
    await page.goto('/');
    const stopwatch = page.locator('.active-stopwatch.not-mobile');
    await expect(stopwatch).toBeVisible();
    await page.waitForTimeout(1000 * timeoutFactor); // eslint-disable-line playwright/no-wait-for-timeout

    await apiCancelStopwatch(request, name, name, 1, {headers});
    await expect(stopwatch).toBeHidden({timeout: 5000 * timeoutFactor});
  });

  // Repro for https://github.com/go-gitea/gitea/pull/36965#issuecomment-4321282667:
  // clicking the sidebar "Start timer" button reportedly produced a blank page.
  // Drives the actual UI button (not the API) so the link-action → JSONRedirect("")
  // → fetchActionDoRedirect("") path is exercised end-to-end.
  test('sidebar start timer button starts stopwatch without blanking the page', async ({page, request}) => {
    const name = `ev-sw-ui-${randomString(8)}`;
    const headers = apiUserHeaders(name);

    await apiCreateUser(request, name);
    await Promise.all([
      loginUser(page, name),
      (async () => {
        await apiCreateRepo(request, {name, headers});
        await apiCreateIssue(request, {owner: name, repo: name, title: 'sidebar start timer test', headers});
      })(),
    ]);
    await page.goto(`/${name}/${name}/issues/1`);

    await page.getByRole('button', {name: 'Start timer'}).click();

    // After the click the page reloads; the sidebar should now show the Stop/Discard
    // controls and the navbar stopwatch icon should appear. If the page blanked,
    // neither of these would be present.
    await expect(page.getByRole('button', {name: 'Stop timer'})).toBeVisible();
    await expect(page.getByRole('button', {name: 'Discard timer'})).toBeVisible();
    await expect(page.locator('.active-stopwatch.not-mobile')).toBeVisible();
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
    await page2.waitForTimeout(1000 * timeoutFactor); // eslint-disable-line playwright/no-wait-for-timeout

    // Logout from page1 — this sends a logout event to all tabs
    await page1.goto('/user/logout');

    // page2 should be redirected via the logout event
    await expect(page2.getByRole('link', {name: 'Sign In'})).toBeVisible({timeout: 5000 * timeoutFactor});

    await context.close();
  });
});
