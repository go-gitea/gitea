import {test, expect} from '@playwright/test';
import {login, apiDeleteOrg} from './utils.ts';

test('create an organization', async ({page}) => {
  const orgName = `e2e-org-${Date.now()}`;
  await login(page);
  await page.goto('/org/create');
  await page.getByLabel('Organization Name').fill(orgName);
  await page.getByRole('button', {name: 'Create Organization'}).click();
  await expect(page).toHaveURL(new RegExp(`/org/${orgName}`));
  // delete via API because of issues related to form-fetch-action
  await apiDeleteOrg(page.request, orgName);
});
