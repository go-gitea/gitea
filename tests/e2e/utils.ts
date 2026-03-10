import {env} from 'node:process';
import {expect} from '@playwright/test';
import type {APIRequestContext, Locator, Page} from '@playwright/test';

export function apiBaseUrl() {
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
      await new Promise((resolve) => globalThis.setTimeout(resolve, 1000 * (attempt + 1) + jitter));
      continue;
    }
    throw new Error(`${label} failed: ${response.status()} ${await response.text()}`);
  }
}

export async function apiCreateRepo(requestContext: APIRequestContext, {name, autoInit = true}: {name: string; autoInit?: boolean}) {
  await apiRetry(() => requestContext.post(`${apiBaseUrl()}/api/v1/user/repos`, {
    headers: apiHeaders(),
    data: {name, auto_init: autoInit},
  }), 'apiCreateRepo');
}

export async function apiDeleteRepo(requestContext: APIRequestContext, owner: string, name: string) {
  await apiRetry(() => requestContext.delete(`${apiBaseUrl()}/api/v1/repos/${owner}/${name}`, {
    headers: apiHeaders(),
  }), 'apiDeleteRepo');
}

export async function apiDeleteOrg(requestContext: APIRequestContext, name: string) {
  await apiRetry(() => requestContext.delete(`${apiBaseUrl()}/api/v1/orgs/${name}`, {
    headers: apiHeaders(),
  }), 'apiDeleteOrg');
}

const testUserPassword = 'password123!AA';

export function apiUserHeaders(username: string) {
  return apiAuthHeader(username, testUserPassword);
}

export async function apiCreateUser(requestContext: APIRequestContext, username: string) {
  await apiRetry(() => requestContext.post(`${apiBaseUrl()}/api/v1/admin/users`, {
    headers: apiHeaders(),
    data: {username, password: testUserPassword, email: `${username}@e2e.gitea.com`, must_change_password: false},
  }), 'apiCreateUser');
}

export async function apiDeleteUser(requestContext: APIRequestContext, username: string) {
  await apiRetry(() => requestContext.delete(`${apiBaseUrl()}/api/v1/admin/users/${username}?purge=true`, {
    headers: apiHeaders(),
  }), 'apiDeleteUser');
}

export async function clickDropdownItem(page: Page, trigger: Locator, itemText: string) {
  await trigger.click();
  await page.getByText(itemText).click();
}

export async function loginUser(page: Page, username: string) {
  return login(page, username, testUserPassword);
}

export async function login(page: Page, username = env.GITEA_TEST_E2E_USER, password = env.GITEA_TEST_E2E_PASSWORD) {
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(username);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', {name: 'Sign In'}).click();
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeHidden();
}

export async function logout(page: Page) {
  await page.context().clearCookies(); // workaround issues related to fomantic dropdown
  await page.goto('/');
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeVisible();
}
