import {test, expect} from '@playwright/test';

test('explore repositories', async ({page}) => {
  await page.goto('/explore/repos');
  await expect(page.getByPlaceholder('Search repos…')).toBeVisible();
  await expect(page.getByRole('link', {name: 'Repositories'})).toBeVisible();
});

test('explore users', async ({page}) => {
  await page.goto('/explore/users');
  await expect(page.getByPlaceholder('Search users…')).toBeVisible();
});

test('explore organizations', async ({page}) => {
  await page.goto('/explore/organizations');
  await expect(page.getByPlaceholder('Search orgs…')).toBeVisible();
});
