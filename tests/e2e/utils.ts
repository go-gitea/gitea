import {env} from 'node:process';
import {expect} from '@playwright/test';
import type {APIRequestContext, Locator, Page} from '@playwright/test';

export function apiBaseUrl() {
  return env.E2E_URL?.replace(/\/$/g, '') || 'http://localhost:3000';
}

export function apiHeaders() {
  return {Authorization: `Basic ${globalThis.btoa(`${env.E2E_USER}:${env.E2E_PASSWORD}`)}`};
}

async function apiRetry(fn: () => Promise<{ok: () => boolean; status: () => number; text: () => Promise<string>}>, label: string) {
  const maxAttempts = 5;
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    const response = await fn();
    if (response.ok()) return;
    if (response.status() === 500 && attempt < maxAttempts - 1) {
      const jitter = Math.random() * 500;
      await new Promise((resolve) => globalThis.setTimeout(resolve, 1000 * (attempt + 1) + jitter));
      continue;
    }
    throw new Error(`${label} failed: ${response.status()} ${await response.text()}`);
  }
}

export async function createRepoApi(requestContext: APIRequestContext, {name, autoInit = true}: {name: string; autoInit?: boolean}) {
  await apiRetry(() => requestContext.post(`${apiBaseUrl()}/api/v1/user/repos`, {
    headers: apiHeaders(),
    data: {name, auto_init: autoInit},
  }), 'createRepoApi');
}

export async function deleteRepoApi(requestContext: APIRequestContext, owner: string, name: string) {
  await apiRetry(() => requestContext.delete(`${apiBaseUrl()}/api/v1/repos/${owner}/${name}`, {
    headers: apiHeaders(),
  }), 'deleteRepoApi');
}

export async function deleteOrgApi(requestContext: APIRequestContext, name: string) {
  await apiRetry(() => requestContext.delete(`${apiBaseUrl()}/api/v1/orgs/${name}`, {
    headers: apiHeaders(),
  }), 'deleteOrgApi');
}

export async function clickDropdownItem(page: Page, trigger: Locator, itemText: string) {
  await trigger.click();
  await page.getByText(itemText).click();
}

export async function login(page: Page) {
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(env.E2E_USER!);
  await page.getByLabel('Password').fill(env.E2E_PASSWORD!);
  await page.getByRole('button', {name: 'Sign In'}).click();
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeHidden();
}

export async function logout(page: Page) {
  await page.context().clearCookies(); // workarkound issues related to fomantic dropdown
  await page.goto('/');
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeVisible();
}
