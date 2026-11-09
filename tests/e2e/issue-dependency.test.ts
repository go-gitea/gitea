import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateIssue, apiDeleteRepo, apiHeaders, randomString} from './utils.ts';

test('dependency dropdown hides existing dependencies', async ({page}) => {
  const repoName = `e2e-issue-dependency-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName, autoInit: false})]);
  const [current, blockedBy, blocking] = await Promise.all(['Current', 'Blocked by', 'Blocking', 'Candidate'].map((title) => apiCreateIssue(page.request, {owner: user, repo: repoName, title})));
  const addDependency = (index: number, dependency: number) => page.request.post(`/api/v1/repos/${user}/${repoName}/issues/${index}/dependencies`, {
    headers: apiHeaders(),
    data: {owner: user, repo: repoName, index: dependency},
  });
  for (const response of await Promise.all([addDependency(current.index, blockedBy.index), addDependency(blocking.index, current.index)])) {
    expect(response.ok()).toBe(true);
  }
  // with cross-repository dependencies the candidates include issues of repos that other tests create and delete
  await page.route('**/dependency/candidates?*', async (route) => {
    const response = await route.fetch();
    const issues: Array<{repository: {full_name: string} | null}> = await response.json();
    await route.fulfill({response, json: issues.filter((issue) => issue.repository?.full_name === `${user}/${repoName}`)});
  });
  await page.goto(`/${user}/${repoName}/issues/${current.index}`);
  const dropdown = page.locator('#new-dependency-drop-list');
  await expect(dropdown.locator('.menu')).toBeAttached(); // created when the dropdown is initialized
  await dropdown.click();
  await expect(dropdown.locator('.menu .item')).toHaveText([/Candidate/]);
  await apiDeleteRepo(page.request, user, repoName);
});
