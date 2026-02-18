import {env} from 'node:process';
import {expect} from '@playwright/test';
import type {APIRequestContext, Locator, Page} from '@playwright/test';

export function apiBaseUrl() {
  return env.E2E_URL?.replace(/\/$/g, '');
}

export function apiHeaders() {
  return {Authorization: `Basic ${globalThis.btoa(`${env.E2E_USER}:${env.E2E_PASSWORD}`)}`};
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

export async function createRepo(page: Page, name: string) {
  await page.goto('/repo/create');
  await page.locator('input[name="repo_name"]').fill(name);
  await page.locator('input[name="auto_init"]').check();
  await page.getByRole('button', {name: 'Create Repository'}).click();
}

export async function deleteRepo(page: Page, owner: string, name: string) {
  await page.goto(`/${owner}/${name}/settings`);
  await page.locator('button[data-modal="#delete-repo-modal"]').click();
  const modal = page.locator('#delete-repo-modal');
  await modal.locator('input[name="repo_name"]').fill(name);
  await modal.getByRole('button', {name: 'Delete Repository'}).click();
  await page.waitForURL('**/');
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

export async function login(page: Page, username = env.E2E_USER, password = env.E2E_PASSWORD) {
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
