import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateFiles, randomString} from './utils.ts';

test('create a pull request from the compare page', async ({page, request}) => {
  const repoName = `e2e-pr-create-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  await Promise.all([
    apiCreateFiles(request, owner, repoName, [{path: 'feat.txt', content: 'feature content\n'}], {branch: 'main', newBranch: 'feat'}),
    apiCreateFiles(request, owner, repoName, [{path: 'release.txt', content: 'release content\n'}], {branch: 'main', newBranch: 'release'}),
    login(page),
  ]);
  // expand=1 renders the PR form directly, skipping the "New Pull Request" toggle click
  await page.goto(`/${owner}/${repoName}/compare/main...feat?expand=1`);

  const title = `e2e-pr-${randomString(8)}`;
  await page.getByPlaceholder('Title').fill(title);
  await page.getByRole('button', {name: 'Create Pull Request'}).click();

  // commit, not full load: the PR title heading is server-rendered, so the assertion can resolve before the heavy diff/timeline finishes
  await page.waitForURL(new RegExp(`/${owner}/${repoName}/pulls/\\d+$`), {waitUntil: 'commit'});
  await expect(page.getByRole('heading', {name: title})).toBeVisible();

  // Saving a title-only edit must succeed without selecting or changing the target branch.
  const titleEditor = page.locator('#issue-title-editor');
  const updatedTitle = `${title}-edited`;
  await page.locator('#issue-title-display').getByRole('button', {name: 'Edit', exact: true}).click();
  await titleEditor.getByRole('textbox').fill(updatedTitle);
  await titleEditor.getByRole('button', {name: 'Save', exact: true}).click();
  await expect(titleEditor).toBeHidden();
  await expect(page.getByRole('heading', {name: updatedTitle})).toBeVisible();
  await expect(page.locator('#pull-desc-display').getByRole('link', {name: 'main', exact: true})).toBeVisible();

  // Saving a different target branch must persist it while preserving the title.
  await page.locator('#issue-title-display').getByRole('button', {name: 'Edit', exact: true}).click();
  await page.locator('#pull-target-branch').click();
  await page.locator('#branch-select').getByText(`${owner}:release`, {exact: true}).click();
  await titleEditor.getByRole('button', {name: 'Save', exact: true}).click();
  await expect(titleEditor).toBeHidden();
  await expect(page.getByRole('heading', {name: updatedTitle})).toBeVisible();
  await expect(page.locator('#pull-desc-display').getByRole('link', {name: 'release', exact: true})).toBeVisible();
});
