import {test, expect} from '@playwright/test';
import {login, apiCreateOrg, apiCreateTeam, apiCreateUser, apiDeleteOrg, randomString} from './utils.ts';

test('create an organization', async ({page}) => {
  const orgName = `e2e-org-${randomString(8)}`;
  await login(page);
  await page.goto('/org/create');
  await page.getByLabel('Organization Name').fill(orgName);
  await page.getByRole('button', {name: 'Create Organization'}).click();
  await expect(page).toHaveURL(new RegExp(`/org/${orgName}`));
  // delete via API because of issues related to form-fetch-action
  await apiDeleteOrg(page.request, orgName);
});

test('add team member search', async ({page, request}) => {
  const orgName = `team-add-${randomString(8)}`;
  const teamName = `team-add-${randomString(8)}`;
  const userName = `team-add-${randomString(8)}`;

  await Promise.all([
    (async () => {
      await apiCreateOrg(request, orgName);
      await apiCreateTeam(request, orgName, teamName);
    })(),
    apiCreateUser(request, userName),
    login(page),
  ]);

  await page.goto(`/org/${orgName}/teams/${teamName}`);
  const input = page.locator('#search-user-box input.prompt');
  await input.fill(userName.slice(-6));
  const result = page.locator('#search-user-box .results .result').first();
  await expect(result).toContainText(userName);
});
