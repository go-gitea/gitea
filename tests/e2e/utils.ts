import {env} from 'node:process';
import {expect} from '@playwright/test';
import type {APIRequestContext, Locator, Page} from '@playwright/test';

/** Generate a random alphanumeric string. */
export function randomString(length: number): string {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let index = 0; index < length; index++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

export const timeoutFactor = Number(env.GITEA_TEST_E2E_TIMEOUT_FACTOR) || 1;

export function baseUrl() {
  return env.GITEA_TEST_E2E_URL?.replace(/\/$/g, '');
}

function apiAuthHeader(username: string, password: string) {
  return {Authorization: `Basic ${globalThis.btoa(`${username}:${password}`)}`};
}

export function apiHeaders() {
  return apiAuthHeader(env.GITEA_TEST_E2E_USER, env.GITEA_TEST_E2E_PASSWORD);
}

async function apiRetry(fn: () => Promise<{ok: () => boolean; status: () => number; text: () => Promise<string>}>, label: string) {
  const maxAttempts = 5;
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    const response = await fn();
    if (response.ok()) return;
    if ([500, 502, 503].includes(response.status()) && attempt < maxAttempts - 1) {
      const jitter = Math.random() * 500;
      await new Promise((resolve) => setTimeout(resolve, 1000 * (attempt + 1) + jitter));
      continue;
    }
    throw new Error(`${label} failed: ${response.status()} ${await response.text()}`);
  }
}

export async function apiCreateRepo(requestContext: APIRequestContext, {name, autoInit = true, headers}: {name: string; autoInit?: boolean; headers?: Record<string, string>}) {
  await apiRetry(() => requestContext.post(`${baseUrl()}/api/v1/user/repos`, {
    headers: headers || apiHeaders(),
    data: {name, auto_init: autoInit},
  }), 'apiCreateRepo');
}

export async function apiCreateOrg(requestContext: APIRequestContext, name: string, {headers}: {headers?: Record<string, string>} = {}) {
  await apiRetry(() => requestContext.post(`${baseUrl()}/api/v1/orgs`, {
    headers: headers || apiHeaders(),
    data: {username: name},
  }), 'apiCreateOrg');
}

export async function apiCreateTeam(requestContext: APIRequestContext, org: string, name: string, {permission = 'read', units = ['repo.code'], headers}: {permission?: string; units?: Array<string>; headers?: Record<string, string>} = {}) {
  await apiRetry(() => requestContext.post(`${baseUrl()}/api/v1/orgs/${org}/teams`, {
    headers: headers || apiHeaders(),
    data: {name, permission, units},
  }), 'apiCreateTeam');
}

export async function apiStartStopwatch(requestContext: APIRequestContext, owner: string, repo: string, issueIndex: number, {headers}: {headers?: Record<string, string>} = {}) {
  await apiRetry(() => requestContext.post(`${baseUrl()}/api/v1/repos/${owner}/${repo}/issues/${issueIndex}/stopwatch/start`, {
    headers: headers || apiHeaders(),
  }), 'apiStartStopwatch');
}

export async function apiCreateFile(requestContext: APIRequestContext, owner: string, repo: string, filepath: string, content: string, {branch, newBranch, message}: {branch?: string; newBranch?: string; message?: string} = {}) {
  await apiRetry(() => requestContext.post(`${baseUrl()}/api/v1/repos/${owner}/${repo}/contents/${filepath}`, {
    headers: apiHeaders(),
    data: {content: Buffer.from(content, 'utf8').toString('base64'), branch, new_branch: newBranch, message},
  }), 'apiCreateFile');
}

export async function apiCreateBranch(requestContext: APIRequestContext, owner: string, repo: string, newBranch: string) {
  await apiRetry(() => requestContext.post(`${baseUrl()}/api/v1/repos/${owner}/${repo}/branches`, {
    headers: apiHeaders(),
    data: {new_branch_name: newBranch},
  }), 'apiCreateBranch');
}

/** Create a PR via API. Returns the PR index for subsequent operations. */
export async function apiCreatePR(requestContext: APIRequestContext, owner: string, repo: string, head: string, base: string, title: string, {headers}: {headers?: Record<string, string>} = {}): Promise<number> {
  let prIndex = 0;
  await apiRetry(async () => {
    const response = await requestContext.post(`${baseUrl()}/api/v1/repos/${owner}/${repo}/pulls`, {
      headers: headers || apiHeaders(),
      data: {head, base, title},
    });
    if (response.ok()) prIndex = (await response.json()).number;
    return response;
  }, 'apiCreatePR');
  return prIndex;
}

/** Create a review on a PR. `event: "COMMENT"` submits immediately without a pending review. */
export async function apiCreateReview(requestContext: APIRequestContext, owner: string, repo: string, index: number, {event = 'COMMENT', body, comments = [], headers}: {event?: string; body?: string; comments?: Array<{path: string; body: string; new_position?: number; old_position?: number}>; headers?: Record<string, string>} = {}) {
  await apiRetry(() => requestContext.post(`${baseUrl()}/api/v1/repos/${owner}/${repo}/pulls/${index}/reviews`, {
    headers: headers || apiHeaders(),
    data: {event, body, comments},
  }), 'apiCreateReview');
}

export async function createProjectColumn(requestContext: APIRequestContext, owner: string, repo: string, projectID: string, title: string) {
  await apiRetry(() => requestContext.post(`${baseUrl()}/${owner}/${repo}/projects/${projectID}/columns/new`, {
    headers: apiHeaders(),
    form: {title},
  }), 'createProjectColumn');
}

export async function apiDeleteRepo(requestContext: APIRequestContext, owner: string, name: string) {
  await apiRetry(() => requestContext.delete(`${baseUrl()}/api/v1/repos/${owner}/${name}`, {
    headers: apiHeaders(),
  }), 'apiDeleteRepo');
}

export async function apiDeleteOrg(requestContext: APIRequestContext, name: string) {
  await apiRetry(() => requestContext.delete(`${baseUrl()}/api/v1/orgs/${name}`, {
    headers: apiHeaders(),
  }), 'apiDeleteOrg');
}

/** Password shared by all test users — used for both API user creation and browser login. */
const testUserPassword = 'e2e-password!aA1';

export function apiUserHeaders(username: string) {
  return apiAuthHeader(username, testUserPassword);
}

export async function apiCreateUser(requestContext: APIRequestContext, username: string) {
  await apiRetry(() => requestContext.post(`${baseUrl()}/api/v1/admin/users`, {
    headers: apiHeaders(),
    data: {username, password: testUserPassword, email: `${username}@${env.GITEA_TEST_E2E_DOMAIN}`, must_change_password: false},
  }), 'apiCreateUser');
}

export async function apiDeleteUser(requestContext: APIRequestContext, username: string) {
  await apiRetry(() => requestContext.delete(`${baseUrl()}/api/v1/admin/users/${username}?purge=true`, {
    headers: apiHeaders(),
  }), 'apiDeleteUser');
}

export async function createProject(
  page: Page,
  {owner, repo, title}: {owner: string; repo: string; title: string},
): Promise<{id: number}> {
  // Navigate to new project page
  await page.goto(`/${owner}/${repo}/projects/new`);

  // Fill in project details
  await page.getByLabel('Title').fill(title);

  // Submit the form
  await page.getByRole('button', {name: 'Create Project'}).click();

  // Wait for redirect to projects list
  await page.waitForURL(new RegExp(`/${owner}/${repo}/projects$`));

  // Extract the project ID from the project link in the list
  const projectLink = page.locator('.milestone-list .milestone-card').filter({hasText: title}).locator('a').first();
  const href = await projectLink.getAttribute('href');
  const match = /\/projects\/(\d+)/.exec(href || '');
  const id = match ? parseInt(match[1]) : 0;

  return {id};
}

export async function apiCreateIssue(
  requestContext: APIRequestContext,
  {owner, repo, title, body, projects, headers}: {
    owner: string;
    repo: string;
    title: string;
    body?: string;
    projects?: number[];
    headers?: Record<string, string>;
  },
): Promise<{index: number}> {
  let result: {index: number} = {index: 0};
  await apiRetry(async () => {
    const response = await requestContext.post(`${baseUrl()}/api/v1/repos/${owner}/${repo}/issues`, {
      headers: headers || apiHeaders(),
      data: {title, body: body || '', projects: projects || []},
    });
    if (response.ok()) {
      const json = await response.json();
      // API returns "number" field for the issue index
      result = {index: json.number};
    }
    return response;
  }, 'apiCreateIssue');
  return result;
}

export async function clickDropdownItem(page: Page, trigger: Locator, itemText: string) {
  await trigger.click();
  await page.getByText(itemText).click();
}

export async function loginUser(page: Page, username: string) {
  return login(page, username, testUserPassword);
}

export async function login(page: Page, username = env.GITEA_TEST_E2E_USER, password = env.GITEA_TEST_E2E_PASSWORD) {
  const response = await page.request.post('/user/login', {
    form: {user_name: username, password},
    maxRedirects: 0,
  });
  const status = response.status();
  if (status !== 302 && status !== 303) throw new Error(`login as ${username} failed: HTTP ${status}`);
}

export async function assertNoJsError(page: Page) {
  await expect(page.locator('.js-global-error')).toHaveCount(0);
}

/* asserts the child has no horizontal inset from its parent — catches padding/border anywhere
 * in between regardless of which element declares it */
export async function assertFlushWithParent(child: Locator, parent: Locator) {
  const [childBox, parentBox] = await Promise.all([child.boundingBox(), parent.boundingBox()]);
  if (!childBox || !parentBox) throw new Error('boundingBox returned null');
  expect(childBox.x).toBe(parentBox.x);
  expect(childBox.width).toBe(parentBox.width);
}

export async function logout(page: Page) {
  await page.context().clearCookies(); // workaround issues related to fomantic dropdown
  await page.goto('/');
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeVisible();
}
