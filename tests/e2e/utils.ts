import {env} from 'node:process';
import {expect} from '@playwright/test';
import type {APIRequestContext, Locator, Page} from '@playwright/test';

export function apiBaseUrl() {
  return env.GITEA_TEST_E2E_URL?.replace(/\/$/g, '');
}

export function apiHeaders() {
  return {Authorization: `Basic ${globalThis.btoa(`${env.GITEA_TEST_E2E_USER}:${env.GITEA_TEST_E2E_PASSWORD}`)}`};
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

export async function apiCreateFile(requestContext: APIRequestContext, owner: string, repo: string, filepath: string, content: string) {
  await apiRetry(() => requestContext.post(`${apiBaseUrl()}/api/v1/repos/${owner}/${repo}/contents/${filepath}`, {
    headers: apiHeaders(),
    data: {content: globalThis.btoa(content)},
  }), 'apiCreateFile');
}

export async function apiCreateBranch(requestContext: APIRequestContext, owner: string, repo: string, branch: string) {
  await apiRetry(() => requestContext.post(`${apiBaseUrl()}/api/v1/repos/${owner}/${repo}/branches`, {
    headers: apiHeaders(),
    data: {new_branch_name: branch},
  }), 'apiCreateBranch');
}

export async function apiCreatePullRequest(requestContext: APIRequestContext, owner: string, repo: string, {title, head, base = 'main'}: {title: string; head: string; base?: string}): Promise<{number: number; head_sha: string}> {
  const response = await requestContext.post(`${apiBaseUrl()}/api/v1/repos/${owner}/${repo}/pulls`, {
    headers: apiHeaders(),
    data: {title, head, base},
  });
  if (!response.ok()) throw new Error(`apiCreatePullRequest failed: ${response.status()} ${await response.text()}`);
  const data = await response.json();
  return {number: data.number, head_sha: data.head.sha};
}

export async function apiSetCommitStatus(requestContext: APIRequestContext, owner: string, repo: string, sha: string, {context, state, description = ''}: {context: string; state: string; description?: string}) {
  await apiRetry(() => requestContext.post(`${apiBaseUrl()}/api/v1/repos/${owner}/${repo}/statuses/${sha}`, {
    headers: apiHeaders(),
    data: {context, state, description, target_url: `${apiBaseUrl()}`},
  }), 'apiSetCommitStatus');
}

export async function apiSetBranchProtection(requestContext: APIRequestContext, owner: string, repo: string, branch: string, {statusCheckContexts = []}: {statusCheckContexts?: string[]} = {}) {
  await apiRetry(() => requestContext.post(`${apiBaseUrl()}/api/v1/repos/${owner}/${repo}/branch_protections`, {
    headers: apiHeaders(),
    data: {
      branch_name: branch,
      enable_status_check: statusCheckContexts.length > 0,
      status_check_contexts: statusCheckContexts,
    },
  }), 'apiSetBranchProtection');
}

export async function clickDropdownItem(page: Page, trigger: Locator, itemText: string) {
  await trigger.click();
  await page.getByText(itemText).click();
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
