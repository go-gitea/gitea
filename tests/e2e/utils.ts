import {env} from 'node:process';
import {expect} from '@playwright/test';
import type {Locator, Page} from '@playwright/test';

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

export async function deleteOrg(page: Page, name: string) {
  await page.goto(`/org/${name}/settings`);
  await page.locator('button[data-modal="#delete-org-modal"]').click();
  const modal = page.locator('#delete-org-modal');
  await modal.locator('input[name="org_name"]').fill(name);
  await modal.getByRole('button', {name: 'Delete This Organization'}).click();
  await page.waitForURL('**/');
}

export async function deleteUser(page: Page, username: string) {
  await page.goto(`/-/admin/users?q=${username}`);
  const userRow = page.locator('tr', {has: page.locator(`a[href="/${username}"]`)});
  await userRow.locator('a[data-tooltip-content="Edit"]').click();
  await page.locator('button[data-modal="#delete-user-modal"]').click();
  const modal = page.locator('#delete-user-modal');
  await modal.locator('input[name="purge"]').check();
  await modal.locator('.ok.button').click();
  await page.waitForURL('**/-/admin/users');
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
