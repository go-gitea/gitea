import {test, expect} from '@playwright/test';
import {login, apiCreateOrg, apiCreateRepo, apiCreateFiles, randomString} from './utils.ts';

test('repo without templates uses the issue and pull request templates of its owner .gitea repo', async ({page, request}) => {
  const org = `e2e-owner-templates-${randomString(8)}`;
  await apiCreateOrg(request, org);
  await Promise.all([apiCreateRepo(request, {name: '.gitea', org}), apiCreateRepo(request, {name: 'repo', org})]);
  await Promise.all([
    apiCreateFiles(request, org, 'repo', [{path: 'feat.txt', content: 'feat\n'}], {branch: 'main', newBranch: 'feat'}),
    login(page),
  ]);

  await page.goto(`/${org}/repo/issues`);
  await page.getByRole('link', {name: 'New Issue'}).click();
  await expect(page).toHaveURL(new RegExp(`/${org}/repo/issues/new\\??$`));

  await apiCreateFiles(request, org, '.gitea', [
    {path: '.gitea/ISSUE_TEMPLATE/form.yaml', content: 'name: Owner form\nabout: From the owner repo\nbody:\n  - type: textarea\n    id: description\n    attributes:\n      label: Description\n      value: owner form value\n'},
    {path: '.gitea/PULL_REQUEST_TEMPLATE.md', content: 'owner pull request template\n'},
  ]);
  await page.goto(`/${org}/repo/issues`);
  await page.getByRole('link', {name: 'New Issue'}).click();
  await page.locator('.segment', {hasText: 'Owner form'}).getByRole('link', {name: 'Get Started'}).click();
  await page.getByPlaceholder('Title').fill('owner template issue');
  await page.getByRole('button', {name: 'Create Issue'}).click();
  await expect(page.locator('.comment .render-content').first()).toContainText('owner form value');

  await page.goto(`/${org}/repo/compare/main...feat?expand=1`);
  await expect(page.getByRole('textbox', {name: 'Leave a comment'})).toHaveValue('owner pull request template\n');
});
