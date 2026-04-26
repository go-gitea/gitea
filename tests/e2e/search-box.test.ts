import {test, expect} from '@playwright/test';
import {apiCreateOrg, apiCreateOrgRepo, apiCreateRepo, apiCreateTeam, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

// search queries use the unique random suffix (slice(-6)) to avoid result collisions
// from concurrent workers' similarly-prefixed names

test('SearchRepoBox renders results and selects on click', async ({page, request}) => {
  const owner = `srb-${randomString(8)}`;
  const orgName = `srb-org-${randomString(8)}`;
  const repoName = `srb-repo-${randomString(8)}`;
  // a non-Owners team is required: the Owners team includes all repos, which hides the add-repo form
  const teamName = `srb-team-${randomString(8)}`;

  await apiCreateUser(request, owner);
  const ownerHeaders = apiUserHeaders(owner);
  await apiCreateOrg(request, orgName, {headers: ownerHeaders});
  await Promise.all([
    apiCreateOrgRepo(request, orgName, repoName, {headers: ownerHeaders}),
    apiCreateTeam(request, orgName, teamName, {headers: ownerHeaders}),
    loginUser(page, owner),
  ]);

  await page.goto(`/org/${orgName}/teams/${teamName}/repositories`);

  const box = page.locator('div[data-global-init="initSearchRepoBox"]');
  const input = box.locator('input.prompt');
  await input.fill(repoName.slice(-6));

  const result = box.locator('.results .result').first();
  await expect(result).toContainText(repoName);
  await result.click();
  await expect(input).toHaveValue(new RegExp(repoName));
});

test('SearchUserBox renders results and selects on click', async ({page, request}) => {
  const owner = `sub-${randomString(8)}`;
  const target = `sub-target-${randomString(8)}`;
  const repoName = `sub-repo-${randomString(8)}`;

  await Promise.all([apiCreateUser(request, owner), apiCreateUser(request, target)]);
  await Promise.all([
    apiCreateRepo(request, {name: repoName, headers: apiUserHeaders(owner)}),
    loginUser(page, owner),
  ]);

  await page.goto(`/${owner}/${repoName}/settings/collaboration`);

  const box = page.locator('#search-user-box');
  const input = box.locator('input.prompt');
  await input.fill(target.slice(-6));

  const result = box.locator('.results .result').first();
  await expect(result).toContainText(target);
  await result.click();
  await expect(input).toHaveValue(target);
});

test('SearchTeamBox renders results and selects on click', async ({page, request}) => {
  const owner = `stb-${randomString(8)}`;
  const orgName = `stb-org-${randomString(8)}`;
  const repoName = `stb-repo-${randomString(8)}`;
  const teamName = `stb-team-${randomString(8)}`;

  await apiCreateUser(request, owner);
  const ownerHeaders = apiUserHeaders(owner);
  await apiCreateOrg(request, orgName, {headers: ownerHeaders});
  await Promise.all([
    apiCreateOrgRepo(request, orgName, repoName, {headers: ownerHeaders}),
    apiCreateTeam(request, orgName, teamName, {headers: ownerHeaders}),
    loginUser(page, owner),
  ]);

  await page.goto(`/${orgName}/${repoName}/settings/collaboration`);

  const box = page.locator('#search-team-box');
  const input = box.locator('input.prompt');
  await input.fill(teamName.slice(-6));

  const result = box.locator('.results .result').first();
  await expect(result).toContainText(teamName);
  await result.click();
  await expect(input).toHaveValue(teamName);
});
