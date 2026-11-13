import {test, expect} from '@playwright/test';

test('explore repositories', async ({page}) => {
  await page.goto('/explore/repos');
  await expect(page.getByPlaceholder('Search repos…')).toBeVisible();
  await expect(page.getByRole('link', {name: 'Repositories'})).toBeVisible();
  await expect(page.getByRole('button', {name: 'Filter', exact: true})).toBeVisible();
  await expect(page.getByRole('button', {name: 'Sort', exact: true})).toBeVisible();
});

test('explore users', async ({page}) => {
  await page.goto('/explore/users');
  await expect(page.getByPlaceholder('Search users…')).toBeVisible();
  await expect(page.getByRole('button', {name: 'Sort', exact: true})).toBeVisible();
});

test('explore organizations', async ({page}) => {
  await page.goto('/explore/organizations');
  await expect(page.getByPlaceholder('Search orgs…')).toBeVisible();
  await expect(page.getByRole('button', {name: 'Sort', exact: true})).toBeVisible();
});
